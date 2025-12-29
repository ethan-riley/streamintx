package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/bluenviron/mediamtx/internal/defs"
	"github.com/bluenviron/mediamtx/internal/stagingdb"
)

func (a *API) onStagingPathsList(ctx *gin.Context) {
	// Get hours parameter (default 72)
	hoursStr := ctx.DefaultQuery("hours", "72")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 72
	}

	records, err := a.StagingDB.GetRecentPaths(hours)
	if err != nil {
		a.writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	data := &defs.APIStagingPathList{
		Items: convertPathRecords(records),
	}

	data.ItemCount = len(data.Items)
	paginate(&data.Items, &data.PageCount, ctx.Query("page"), ctx.Query("itemsPerPage"))

	ctx.JSON(http.StatusOK, data)
}

func (a *API) onStagingPathsGet(ctx *gin.Context) {
	name, ok := paramName(ctx)
	if !ok {
		a.writeError(ctx, http.StatusBadRequest, nil)
		return
	}

	records, err := a.StagingDB.GetPathsByName(name)
	if err != nil {
		a.writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	data := &defs.APIStagingPathList{
		Items: convertPathRecords(records),
	}

	data.ItemCount = len(data.Items)
	paginate(&data.Items, &data.PageCount, ctx.Query("page"), ctx.Query("itemsPerPage"))

	ctx.JSON(http.StatusOK, data)
}

func (a *API) onStagingConnectionsList(ctx *gin.Context) {
	// Get hours parameter (default 72)
	hoursStr := ctx.DefaultQuery("hours", "72")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 72
	}

	// Optional path filter
	pathFilter := ctx.Query("path")

	var records []stagingdb.ConnectionRecord
	if pathFilter != "" {
		records, err = a.StagingDB.GetConnectionsByPath(pathFilter)
	} else {
		records, err = a.StagingDB.GetRecentConnections(hours)
	}
	if err != nil {
		a.writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	data := &defs.APIStagingConnectionList{
		Items: convertConnectionRecords(records),
	}

	data.ItemCount = len(data.Items)
	paginate(&data.Items, &data.PageCount, ctx.Query("page"), ctx.Query("itemsPerPage"))

	ctx.JSON(http.StatusOK, data)
}

func (a *API) onStagingConnectionsGet(ctx *gin.Context) {
	pathName := ctx.Param("id")

	// Get connections by path name
	records, err := a.StagingDB.GetConnectionsByPath(pathName)
	if err != nil {
		a.writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	data := &defs.APIStagingConnectionList{
		Items: convertConnectionRecords(records),
	}

	data.ItemCount = len(data.Items)
	paginate(&data.Items, &data.PageCount, ctx.Query("page"), ctx.Query("itemsPerPage"))

	ctx.JSON(http.StatusOK, data)
}

func (a *API) onStagingStats(ctx *gin.Context) {
	pathCount, connCount, err := a.StagingDB.Stats()
	if err != nil {
		a.writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Get active counts (paths/connections without closed_time)
	activePaths, err := a.StagingDB.GetRecentPaths(72)
	if err != nil {
		a.writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	activeConns, err := a.StagingDB.GetRecentConnections(72)
	if err != nil {
		a.writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	var activePathCount, activeConnCount int64
	for _, p := range activePaths {
		if p.ClosedTime == nil {
			activePathCount++
		}
	}
	for _, c := range activeConns {
		if c.ClosedTime == nil {
			activeConnCount++
		}
	}

	ctx.JSON(http.StatusOK, &defs.APIStagingStats{
		TotalPaths:        pathCount,
		ActivePaths:       activePathCount,
		TotalConnections:  connCount,
		ActiveConnections: activeConnCount,
	})
}

func convertPathRecords(records []stagingdb.PathRecord) []*defs.APIStagingPath {
	items := make([]*defs.APIStagingPath, len(records))
	for i, r := range records {
		items[i] = &defs.APIStagingPath{
			ID:            r.ID,
			Name:          r.Name,
			ConfName:      r.ConfName,
			SourceType:    r.SourceType,
			SourceID:      r.SourceID,
			Ready:         r.Ready,
			ReadyTime:     r.ReadyTime,
			ClosedTime:    r.ClosedTime,
			Tracks:        r.Tracks,
			BytesReceived: r.BytesReceived,
			BytesSent:     r.BytesSent,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
		}
	}
	return items
}

func convertConnectionRecords(records []stagingdb.ConnectionRecord) []*defs.APIStagingConnection {
	items := make([]*defs.APIStagingConnection, len(records))
	for i, r := range records {
		items[i] = &defs.APIStagingConnection{
			ID:            r.ID,
			ConnID:        r.ConnID,
			ConnType:      r.ConnType,
			Created:       r.Created,
			ClosedTime:    r.ClosedTime,
			RemoteAddr:    r.RemoteAddr,
			State:         r.State,
			Path:          r.PathName,
			Query:         r.Query,
			BytesReceived: r.BytesReceived,
			BytesSent:     r.BytesSent,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
		}
	}
	return items
}
