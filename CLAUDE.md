# StreaminTX - Renamed from MediaMTX

## Project Renaming Summary

This project was renamed from "mediamtx" to "streamintx". Below is a record of all changes made during the renaming session.

## Files Modified

### Build Configuration
- `scripts/binaries.mk`
  - Changed `BINARY_NAME = mediamtx` → `BINARY_NAME = streamintx`
  - Changed all `mediamtx.yml` references → `streamintx.yml`

### Configuration Files
- Renamed `mediamtx.yml` → `streamintx.yml`

### Core Application
- `internal/core/core.go`
  - Default config paths: `streamintx.yml`
  - System paths: `/usr/local/etc/streamintx.yml`, `/usr/etc/streamintx.yml`, `/etc/streamintx/streamintx.yml`
  - Help text updated to reference `streamintx.yml`

- `internal/conf/conf.go`
  - Log file: `streamintx.log`
  - Syslog prefix: `streamintx`
  - JWT claim key: `streamintx_permissions`

- `internal/core/upgrade.go`
  - Git repo URL: `https://github.com/bluenviron/streamintx`
  - Download URL pattern updated for `streamintx_*` archives
  - Executable name: `streamintx`
  - Success message: "StreaminTX upgraded successfully..."

- `internal/core/versiongetter/main.go`
  - Log message: "getting streamintx version..."

### Test Files
- `internal/conf/conf_test.go`
  - Updated references from `../../mediamtx.yml` → `../../streamintx.yml`

### Docker Files
- `docker/standard.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

- `docker/ffmpeg.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

- `docker/rpi.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

- `docker/ffmpeg-rpi.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

### Generated Files
- `internal/core/VERSION` - Created with `v0.0.1`

## Build Instructions

Before building, you need to generate embedded files:

```bash
# Option 1: Run go generate (downloads hls.js, creates VERSION from git)
go generate ./...

# Option 2: Manual (if not a git repo)
# VERSION file already created at internal/core/VERSION
cd internal/servers/hls && go run ./hlsjsdownloader && cd ../../..

# Build the binary
go build -o streamintx .
```

## Remaining Work (Optional)

The following items were NOT changed and may need updating if you want a complete rebrand:

1. **Go module path** (`go.mod`): Still `github.com/bluenviron/mediamtx`
   - Would require updating all import statements across ~225 files

2. **Internal package imports**: All files still import from `github.com/bluenviron/mediamtx/internal/...`

3. **Auth realm** (`internal/playback/server.go`): `Basic realm="mediamtx"`

4. **Temp directory prefixes** in tests: `mediamtx-playback`, `mediamtx-playback-fmp4-`

5. **Docker repository** (`scripts/dockerhub.mk`): `bluenviron/mediamtx`

6. **Documentation**: README.md and docs/ folder references

7. **GitHub workflow files**: `.github/workflows/`

## Staging Database Feature

A SQLite-based staging database has been added to log all stream information for the last 72 hours (configurable). This feature tracks:

### Path Events
- Path name, config name, source type, source ID
- Ready time (when stream starts)
- Closed time (when stream ends)
- Tracks/codecs information
- Bytes received/sent statistics

### Connection Events (RTMP/RTMPS)
- Connection ID, type (rtmp/rtmps)
- Created time, closed time
- Remote address
- State (idle, streaming, published, timeout)
- Path name and query parameters
- Authenticated user (extracted from query params `?user=xxx`)
- Bytes received/sent statistics

### State Management

The staging database tracks 4 states for paths and connections:

| State | Description |
|-------|-------------|
| `idle` | Connection opened but not yet publishing/reading (onDemand waiting) |
| `streaming` | Actively transmitting (no closedTime) |
| `published` | Completed normally by client (has closedTime) |
| `timeout` | Connection lost unexpectedly (crash/disconnect) |

### Crash Recovery

On startup, the staging database automatically recovers stale records:
- Connections in `streaming`/`publish`/`read` state without closedTime → marked as `timeout`
- Paths from publisher sources without closedTime → marked as `timeout`
- Only recovers publisher sources: `rtmpConn`, `rtmpsConn`, `webRTCSession`, `rtspSession`, `srtConn`
- Does NOT recover external sources (rtspSource, hlsSource, etc.)

### User Tracking

The authenticated user is extracted from RTMP query parameters:
- URL format: `rtmp://host/path?user=xxx&pass=xxx` → user field = "xxx"
- Note: URL userinfo format (`rtmp://user:pass@host/path`) is NOT supported because RTMP clients strip credentials before transmission

### Configuration Options
```yaml
# Enable the staging database
stagingDB: yes
# Path to the SQLite database file
stagingDBPath: streamintx_staging.db
# How long to retain data (default: 72 hours)
stagingDBRetentionPeriod: 72h
```

### New Files
- `internal/stagingdb/stagingdb.go` - SQLite database module with automatic cleanup

### Modified Files
- `internal/conf/conf.go` - Added configuration options
- `internal/core/core.go` - Added stagingDB initialization and lifecycle
- `internal/core/path_manager.go` - Hook path ready/not ready events
- `internal/servers/rtmp/server.go` - Pass stagingDB to connections
- `internal/servers/rtmp/conn.go` - Record connection lifecycle events

### Dependencies
- Added `github.com/mattn/go-sqlite3` (CGO-based SQLite driver)

### API Endpoints (Staging Database)

The following new API endpoints are available when the staging database is enabled:

#### GET /v3/staging/paths/list
List all path records from the staging database.

Query parameters:
- `hours` (optional, default: 72) - Number of hours to look back
- `page` (optional) - Page number for pagination
- `itemsPerPage` (optional) - Items per page

Example:
```bash
curl localhost:9997/v3/staging/paths/list | jq
curl "localhost:9997/v3/staging/paths/list?hours=24" | jq
```

#### GET /v3/staging/paths/get/{name}
Get all records for a specific path name.

Example:
```bash
curl localhost:9997/v3/staging/paths/get/C4_test_01 | jq
```

#### GET /v3/staging/connections/list
List all connection records from the staging database.

Query parameters:
- `hours` (optional, default: 72) - Number of hours to look back
- `path` (optional) - Filter by path name
- `page` (optional) - Page number for pagination
- `itemsPerPage` (optional) - Items per page

Example:
```bash
curl localhost:9997/v3/staging/connections/list | jq
curl "localhost:9997/v3/staging/connections/list?path=C4_test_01" | jq
```

#### GET /v3/staging/connections/get/{path}
Get all connections for a specific path.

Example:
```bash
curl localhost:9997/v3/staging/connections/get/C4_test_01 | jq
```

#### GET /v3/staging/stats
Get statistics about the staging database.

Example:
```bash
curl localhost:9997/v3/staging/stats | jq
```

Response:
```json
{
  "totalPaths": 150,
  "activePaths": 2,
  "totalConnections": 200,
  "activeConnections": 3
}
```

### Sample Response - Path with closedTime

```json
{
  "itemCount": 1,
  "pageCount": 1,
  "items": [
    {
      "id": 1,
      "name": "C4_test_01",
      "confName": "~^C4",
      "sourceType": "rtmpConn",
      "sourceId": "0afd7da4-ac01-4a49-8f08-a5290b0e2041",
      "ready": false,
      "readyTime": "2025-12-29T14:55:49.156291745Z",
      "closedTime": "2025-12-29T15:30:22.123456789Z",
      "state": "published",
      "tracks": ["H264"],
      "bytesReceived": 331969114,
      "bytesSent": 0,
      "createdAt": "2025-12-29T14:55:49Z",
      "updatedAt": "2025-12-29T15:30:22Z"
    }
  ]
}
```

### Sample Response - Connection with user

```json
{
  "itemCount": 1,
  "pageCount": 1,
  "items": [
    {
      "id": 2,
      "connId": "6b80e024-02e4-4266-ad55-dec7838e953e",
      "connType": "rtmp",
      "created": "2025-12-29T17:12:58.75865419-03:00",
      "closedTime": "2025-12-29T17:15:30.123456789-03:00",
      "remoteAddr": "192.168.1.111:58703",
      "state": "published",
      "path": "testuser_stream",
      "query": "user=c4user&pass=xxx",
      "user": "c4user",
      "bytesReceived": 17662199,
      "bytesSent": 3467,
      "createdAt": "2025-12-29T20:12:58Z",
      "updatedAt": "2025-12-29T20:15:30Z"
    }
  ]
}
```

## Notes

- The upgrade feature (`--upgrade` flag) now points to `github.com/bluenviron/streamintx` - update this URL when you create your own repository
- The binary will be named `streamintx` when compiled
- Config file should be named `streamintx.yml`
- Log file defaults to `streamintx.log`
- Staging database file defaults to `streamintx_staging.db`
