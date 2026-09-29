package httptransport

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"net/http"
	"strconv"
	"time"
)

func (h *AgentHandlers) events(c *gin.Context) {
	project, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	run, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("runId", "uuid"))
		return
	}
	svc := batch.Service{Pool: h.Pool}
	user := principalFrom(c).UserID
	if _, err = svc.Get(c.Request.Context(), user, project, run); err != nil {
		respond{}.error(c, err)
		return
	}
	after, _ := strconv.ParseInt(c.GetHeader("Last-Event-ID"), 10, 64)
	if value := c.Query("after"); value != "" {
		after, err = strconv.ParseInt(value, 10, 64)
		if err != nil || after < 0 {
			respond{}.error(c, apierrors.Fields("after", "invalid"))
			return
		}
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if !liveAuthority(c.Request.Context(), h.Pool, principalFrom(c)) {
			return
		}
		state, err := svc.Get(c.Request.Context(), user, project, run)
		if err != nil {
			return
		}
		rows, err := h.Pool.Query(c.Request.Context(), `SELECT seq,type,payload FROM run_events WHERE project_id=$1 AND run_id=$2 AND visibility='public' AND seq>$3 ORDER BY seq LIMIT 100`, project, run, after)
		if err != nil {
			return
		}
		for rows.Next() {
			var seq int64
			var kind string
			var payload []byte
			if rows.Scan(&seq, &kind, &payload) != nil {
				rows.Close()
				return
			}
			var data any
			_ = json.Unmarshal(payload, &data)
			c.Render(-1, runSSE{ID: strconv.FormatInt(seq, 10), Event: kind, Data: data})
			after = seq
		}
		rows.Close()
		c.SSEvent("run.changed", state)
		c.Writer.Flush()
		if state.State == "succeeded" || state.State == "failed" || state.State == "cancelled" || state.State == "waiting_human" {
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

type runSSE struct {
	ID, Event string
	Data      any
}

func (e runSSE) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
}
func (e runSSE) Render(w http.ResponseWriter) error {
	raw, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("id: " + e.ID + "\nevent: " + e.Event + "\ndata: " + string(raw) + "\n\n"))
	return err
}
