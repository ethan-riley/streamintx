// Package stagingdb provides a SQLite-based staging database for storing stream information
// with automatic cleanup of data older than a configurable retention period (default 72 hours).
package stagingdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/bluenviron/mediamtx/internal/logger"
)

// Connection/Path states
const (
	// StateIdle - connection opened but not yet publishing/reading (onDemand waiting)
	StateIdle = "idle"
	// StateStreaming - actively transmitting (no closedTime)
	StateStreaming = "streaming"
	// StatePublished - completed normally by client (has closedTime)
	StatePublished = "published"
	// StateTimeout - connection lost unexpectedly (crash/disconnect)
	StateTimeout = "timeout"
)

// PathRecord represents a recorded path/stream event
type PathRecord struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	ConfName      string     `json:"confName"`
	SourceType    string     `json:"sourceType"`
	SourceID      string     `json:"sourceId"`
	Ready         bool       `json:"ready"`
	ReadyTime     *time.Time `json:"readyTime"`
	ClosedTime    *time.Time `json:"closedTime"`
	State         string     `json:"state"` // streaming, published, idle, timeout
	Tracks        []string   `json:"tracks"`
	BytesReceived uint64     `json:"bytesReceived"`
	BytesSent     uint64     `json:"bytesSent"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ConnectionRecord represents a recorded connection event
type ConnectionRecord struct {
	ID            int64      `json:"id"`
	ConnID        uuid.UUID  `json:"connId"`
	ConnType      string     `json:"connType"` // rtmp, rtmps, rtsp, rtsps, srt, webrtc
	Created       time.Time  `json:"created"`
	ClosedTime    *time.Time `json:"closedTime"`
	RemoteAddr    string     `json:"remoteAddr"`
	State         string     `json:"state"` // idle, read, publish
	PathName      string     `json:"path"`
	Query         string     `json:"query"`
	User          string     `json:"user"` // authenticated user
	BytesReceived uint64     `json:"bytesReceived"`
	BytesSent     uint64     `json:"bytesSent"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// StagingDBParent is implemented by the parent
type StagingDBParent interface {
	logger.Writer
}

// StagingDB manages the SQLite staging database
type StagingDB struct {
	DBPath          string
	RetentionPeriod time.Duration // Default 72 hours
	CleanupInterval time.Duration // Default 1 hour
	Parent          StagingDBParent

	ctx       context.Context
	ctxCancel func()
	db        *sql.DB
	mutex     sync.RWMutex
	wg        sync.WaitGroup
}

// Initialize sets up the database and starts the cleanup routine
func (s *StagingDB) Initialize() error {
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())

	if s.RetentionPeriod == 0 {
		s.RetentionPeriod = 72 * time.Hour
	}
	if s.CleanupInterval == 0 {
		s.CleanupInterval = 1 * time.Hour
	}

	db, err := sql.Open("sqlite3", s.DBPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return err
	}
	s.db = db

	if err := s.createTables(); err != nil {
		return err
	}

	// Recover stale connections/paths from previous crash
	s.recoverStaleRecords()

	s.Log(logger.Info, "staging database initialized at %s (retention: %v)", s.DBPath, s.RetentionPeriod)

	// Start cleanup routine
	s.wg.Add(1)
	go s.cleanupRoutine()

	return nil
}

// Close shuts down the database
func (s *StagingDB) Close() {
	s.ctxCancel()
	s.wg.Wait()
	if s.db != nil {
		s.db.Close()
	}
	s.Log(logger.Info, "staging database closed")
}

// Log implements logger.Writer
func (s *StagingDB) Log(level logger.Level, format string, args ...any) {
	s.Parent.Log(level, "[stagingdb] "+format, args...)
}

func (s *StagingDB) createTables() error {
	// Create paths table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS paths (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			conf_name TEXT NOT NULL,
			source_type TEXT,
			source_id TEXT,
			ready BOOLEAN DEFAULT FALSE,
			ready_time DATETIME,
			closed_time DATETIME,
			state TEXT DEFAULT 'idle',
			tracks TEXT,
			bytes_received INTEGER DEFAULT 0,
			bytes_sent INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create connections table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS connections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conn_id TEXT NOT NULL,
			conn_type TEXT NOT NULL,
			created DATETIME NOT NULL,
			closed_time DATETIME,
			remote_addr TEXT NOT NULL,
			state TEXT DEFAULT 'idle',
			path_name TEXT,
			query TEXT,
			user TEXT,
			bytes_received INTEGER DEFAULT 0,
			bytes_sent INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Add columns if they don't exist (for existing databases)
	_, _ = s.db.Exec(`ALTER TABLE connections ADD COLUMN user TEXT`)
	_, _ = s.db.Exec(`ALTER TABLE paths ADD COLUMN state TEXT DEFAULT 'idle'`)

	// Create indexes for faster queries
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_paths_name ON paths(name)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_paths_created_at ON paths(created_at)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_connections_conn_id ON connections(conn_id)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_connections_created_at ON connections(created_at)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_connections_path_name ON connections(path_name)`)
	if err != nil {
		return err
	}

	return nil
}

// RecordPathReady records when a path becomes ready (stream starts)
func (s *StagingDB) RecordPathReady(name, confName, sourceType, sourceID string, readyTime time.Time, tracks []string) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tracksJSON, _ := json.Marshal(tracks)

	result, err := s.db.Exec(`
		INSERT INTO paths (name, conf_name, source_type, source_id, ready, ready_time, state, tracks, created_at, updated_at)
		VALUES (?, ?, ?, ?, TRUE, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, name, confName, sourceType, sourceID, readyTime, StateStreaming, string(tracksJSON))
	if err != nil {
		s.Log(logger.Error, "failed to record path ready: %v", err)
		return 0, err
	}

	id, _ := result.LastInsertId()
	s.Log(logger.Debug, "recorded path ready: %s (id=%d, state=%s)", name, id, StateStreaming)
	return id, nil
}

// RecordPathClosed records when a path is closed (stream ends normally)
func (s *StagingDB) RecordPathClosed(id int64, closedTime time.Time, bytesReceived, bytesSent uint64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(`
		UPDATE paths
		SET ready = FALSE, closed_time = ?, state = ?, bytes_received = ?, bytes_sent = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, closedTime, StatePublished, bytesReceived, bytesSent, id)
	if err != nil {
		s.Log(logger.Error, "failed to record path closed: %v", err)
		return err
	}

	s.Log(logger.Debug, "recorded path closed: id=%d (state=%s)", id, StatePublished)
	return nil
}

// UpdatePathStats updates the bytes received/sent for a path
func (s *StagingDB) UpdatePathStats(id int64, bytesReceived, bytesSent uint64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(`
		UPDATE paths
		SET bytes_received = ?, bytes_sent = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, bytesReceived, bytesSent, id)
	return err
}

// RecordConnectionOpened records when a connection is opened
func (s *StagingDB) RecordConnectionOpened(connID uuid.UUID, connType string, created time.Time, remoteAddr string) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	result, err := s.db.Exec(`
		INSERT INTO connections (conn_id, conn_type, created, remote_addr, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'idle', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, connID.String(), connType, created, remoteAddr)
	if err != nil {
		s.Log(logger.Error, "failed to record connection opened: %v", err)
		return 0, err
	}

	id, _ := result.LastInsertId()
	s.Log(logger.Debug, "recorded connection opened: %s (id=%d, type=%s)", connID.String(), id, connType)
	return id, nil
}

// RecordConnectionStateChange records when a connection changes state
// When a connection starts publishing or reading, it transitions to "streaming" state
func (s *StagingDB) RecordConnectionStateChange(id int64, state, pathName, query, user string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Map legacy states to new states
	// publish/read -> streaming (active transmission)
	mappedState := state
	if state == "publish" || state == "read" {
		mappedState = StateStreaming
	}

	_, err := s.db.Exec(`
		UPDATE connections
		SET state = ?, path_name = ?, query = ?, user = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, mappedState, pathName, query, user, id)
	if err != nil {
		s.Log(logger.Error, "failed to record connection state change: %v", err)
		return err
	}

	s.Log(logger.Debug, "recorded connection state change: id=%d, state=%s, path=%s, user=%s", id, mappedState, pathName, user)
	return nil
}

// RecordConnectionClosed records when a connection is closed normally
func (s *StagingDB) RecordConnectionClosed(id int64, closedTime time.Time, bytesReceived, bytesSent uint64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(`
		UPDATE connections
		SET closed_time = ?, state = ?, bytes_received = ?, bytes_sent = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, closedTime, StatePublished, bytesReceived, bytesSent, id)
	if err != nil {
		s.Log(logger.Error, "failed to record connection closed: %v", err)
		return err
	}

	s.Log(logger.Debug, "recorded connection closed: id=%d (state=%s)", id, StatePublished)
	return nil
}

// GetRecentPaths returns paths from the last n hours
func (s *StagingDB) GetRecentPaths(hours int) ([]PathRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, name, conf_name, source_type, source_id, ready, ready_time, closed_time,
		       state, tracks, bytes_received, bytes_sent, created_at, updated_at
		FROM paths
		WHERE created_at >= datetime('now', ?)
		ORDER BY created_at DESC
	`, fmt.Sprintf("-%d hours", hours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []PathRecord
	for rows.Next() {
		var r PathRecord
		var tracksJSON string
		var readyTime, closedTime sql.NullTime
		var sourceType, sourceID, state sql.NullString

		err := rows.Scan(&r.ID, &r.Name, &r.ConfName, &sourceType, &sourceID,
			&r.Ready, &readyTime, &closedTime, &state, &tracksJSON,
			&r.BytesReceived, &r.BytesSent, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			continue
		}

		if sourceType.Valid {
			r.SourceType = sourceType.String
		}
		if sourceID.Valid {
			r.SourceID = sourceID.String
		}
		if readyTime.Valid {
			r.ReadyTime = &readyTime.Time
		}
		if closedTime.Valid {
			r.ClosedTime = &closedTime.Time
		}
		if state.Valid {
			r.State = state.String
		}
		json.Unmarshal([]byte(tracksJSON), &r.Tracks)

		records = append(records, r)
	}

	return records, nil
}

// GetRecentConnections returns connections from the last n hours
func (s *StagingDB) GetRecentConnections(hours int) ([]ConnectionRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, conn_id, conn_type, created, closed_time, remote_addr, state,
		       path_name, query, user, bytes_received, bytes_sent, created_at, updated_at
		FROM connections
		WHERE created_at >= datetime('now', ?)
		ORDER BY created_at DESC
	`, fmt.Sprintf("-%d hours", hours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ConnectionRecord
	for rows.Next() {
		var r ConnectionRecord
		var connIDStr string
		var closedTime sql.NullTime
		var pathName, query, user sql.NullString

		err := rows.Scan(&r.ID, &connIDStr, &r.ConnType, &r.Created, &closedTime,
			&r.RemoteAddr, &r.State, &pathName, &query, &user,
			&r.BytesReceived, &r.BytesSent, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			continue
		}

		r.ConnID, _ = uuid.Parse(connIDStr)
		if closedTime.Valid {
			r.ClosedTime = &closedTime.Time
		}
		if pathName.Valid {
			r.PathName = pathName.String
		}
		if query.Valid {
			r.Query = query.String
		}
		if user.Valid {
			r.User = user.String
		}

		records = append(records, r)
	}

	return records, nil
}

// GetPathsByName returns all records for a specific path name
func (s *StagingDB) GetPathsByName(name string) ([]PathRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, name, conf_name, source_type, source_id, ready, ready_time, closed_time,
		       state, tracks, bytes_received, bytes_sent, created_at, updated_at
		FROM paths
		WHERE name = ?
		ORDER BY created_at DESC
	`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []PathRecord
	for rows.Next() {
		var r PathRecord
		var tracksJSON string
		var readyTime, closedTime sql.NullTime
		var sourceType, sourceID, state sql.NullString

		err := rows.Scan(&r.ID, &r.Name, &r.ConfName, &sourceType, &sourceID,
			&r.Ready, &readyTime, &closedTime, &state, &tracksJSON,
			&r.BytesReceived, &r.BytesSent, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			continue
		}

		if sourceType.Valid {
			r.SourceType = sourceType.String
		}
		if sourceID.Valid {
			r.SourceID = sourceID.String
		}
		if readyTime.Valid {
			r.ReadyTime = &readyTime.Time
		}
		if closedTime.Valid {
			r.ClosedTime = &closedTime.Time
		}
		if state.Valid {
			r.State = state.String
		}
		json.Unmarshal([]byte(tracksJSON), &r.Tracks)

		records = append(records, r)
	}

	return records, nil
}

// GetConnectionsByPath returns all connections for a specific path
func (s *StagingDB) GetConnectionsByPath(pathName string) ([]ConnectionRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, conn_id, conn_type, created, closed_time, remote_addr, state,
		       path_name, query, user, bytes_received, bytes_sent, created_at, updated_at
		FROM connections
		WHERE path_name = ?
		ORDER BY created_at DESC
	`, pathName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ConnectionRecord
	for rows.Next() {
		var r ConnectionRecord
		var connIDStr string
		var closedTime sql.NullTime
		var pathNameN, query, user sql.NullString

		err := rows.Scan(&r.ID, &connIDStr, &r.ConnType, &r.Created, &closedTime,
			&r.RemoteAddr, &r.State, &pathNameN, &query, &user,
			&r.BytesReceived, &r.BytesSent, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			continue
		}

		r.ConnID, _ = uuid.Parse(connIDStr)
		if closedTime.Valid {
			r.ClosedTime = &closedTime.Time
		}
		if pathNameN.Valid {
			r.PathName = pathNameN.String
		}
		if query.Valid {
			r.Query = query.String
		}
		if user.Valid {
			r.User = user.String
		}

		records = append(records, r)
	}

	return records, nil
}

func (s *StagingDB) cleanupRoutine() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *StagingDB) cleanup() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoff := time.Now().Add(-s.RetentionPeriod)

	// Delete old paths
	result, err := s.db.Exec(`DELETE FROM paths WHERE created_at < ?`, cutoff)
	if err != nil {
		s.Log(logger.Error, "cleanup paths failed: %v", err)
	} else {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			s.Log(logger.Debug, "cleaned up %d old path records", rows)
		}
	}

	// Delete old connections
	result, err = s.db.Exec(`DELETE FROM connections WHERE created_at < ?`, cutoff)
	if err != nil {
		s.Log(logger.Error, "cleanup connections failed: %v", err)
	} else {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			s.Log(logger.Debug, "cleaned up %d old connection records", rows)
		}
	}
}

// recoverStaleRecords marks any unclosed connections/paths as timeout on startup
// This handles the case where the server crashed and connections weren't properly closed
func (s *StagingDB) recoverStaleRecords() {
	now := time.Now()

	// Recover stale connections (those without closedTime that are in streaming state)
	// Only recover connections from publishers (not external sources)
	result, err := s.db.Exec(`
		UPDATE connections
		SET state = ?, closed_time = ?, updated_at = CURRENT_TIMESTAMP
		WHERE closed_time IS NULL AND state IN ('streaming', 'publish', 'read')
	`, StateTimeout, now)
	if err != nil {
		s.Log(logger.Error, "failed to recover stale connections: %v", err)
	} else {
		rows, _ := result.RowsAffected()
		if rows > 0 {
			s.Log(logger.Warn, "recovered %d stale connections (marked as timeout)", rows)
		}
	}

	// Recover stale paths from publisher sources only
	// Source types from publishers: rtmpConn, rtmpsConn, webRTCSession, rtspSession, srtConn
	// Do NOT recover paths from external sources like: rtspSource, rtmpSource, hlsSource, etc.
	publisherSourceTypes := []string{
		"rtmpConn", "rtmpsConn", "webRTCSession", "rtspSession", "rtspsSession", "srtConn",
	}
	for _, sourceType := range publisherSourceTypes {
		result, err = s.db.Exec(`
			UPDATE paths
			SET state = ?, ready = FALSE, closed_time = ?, updated_at = CURRENT_TIMESTAMP
			WHERE closed_time IS NULL AND ready = TRUE AND source_type = ?
		`, StateTimeout, now, sourceType)
		if err != nil {
			s.Log(logger.Error, "failed to recover stale paths for %s: %v", sourceType, err)
		} else {
			rows, _ := result.RowsAffected()
			if rows > 0 {
				s.Log(logger.Warn, "recovered %d stale paths from %s (marked as timeout)", rows, sourceType)
			}
		}
	}
}

// Stats returns database statistics
func (s *StagingDB) Stats() (pathCount, connectionCount int64, err error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	err = s.db.QueryRow(`SELECT COUNT(*) FROM paths`).Scan(&pathCount)
	if err != nil {
		return
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM connections`).Scan(&connectionCount)
	return
}

