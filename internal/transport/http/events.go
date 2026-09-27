// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// eventBus wakes SSE subscribers; durability lives in project_events (04
// §5: outbox 只负责唤醒，消费者按 seq 从持久事件重放).
type eventBus struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func newEventBus() *eventBus {
	return &eventBus{subs: map[string]map[chan struct{}]struct{}{}}
}

func (b *eventBus) subscribe(projectID string) (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	if b.subs[projectID] == nil {
		b.subs[projectID] = map[chan struct{}]struct{}{}
	}
	b.subs[projectID][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs[projectID], ch)
		if len(b.subs[projectID]) == 0 {
			delete(b.subs, projectID)
		}
		b.mu.Unlock()
	}
}

func (b *eventBus) publish(projectID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[projectID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// EventRow is the SSE payload projection of project_events.
type EventRow struct {
	EventID    uuid.UUID `json:"eventId"`
	ProjectID  uuid.UUID `json:"projectId"`
	Seq        int64     `json:"seq"`
	Type       string    `json:"type"`
	ObjectType string    `json:"objectType"`
	ObjectID   string    `json:"objectId"`
	Version    *int64    `json:"version"`
	OccurredAt time.Time `json:"occurredAt"`
}

// SSEHandlers serves GET /projects/{p}/events and the notification
// endpoints (06 §4).
type SSEHandlers struct {
	Pool *pgxpool.Pool
	Bus  *eventBus
}

func NewSSEHandlers(pool *pgxpool.Pool) *SSEHandlers {
	return &SSEHandlers{Pool: pool, Bus: newEventBus()}
}

// Publish wakes subscribers for one project (outbox deliverer calls this).
func (h *SSEHandlers) Publish(projectID string) { h.Bus.publish(projectID) }

func (h *SSEHandlers) fetch(ctx context.Context, projectID uuid.UUID, after int64, limit int) ([]EventRow, error) {
	rows, err := h.Pool.Query(ctx, `
		SELECT event_id, project_id, seq, type, object_type, object_id, version, occurred_at
		FROM project_events WHERE project_id=$1 AND seq>$2 ORDER BY seq LIMIT $3`,
		projectID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var row EventRow
		if err := rows.Scan(&row.EventID, &row.ProjectID, &row.Seq, &row.Type, &row.ObjectType,
			&row.ObjectID, &row.Version, &row.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Subscribe implements the SSE stream: membership is checked on connect and
// on every wake; replay from the cursor, then live with 15s heartbeats;
// stale cursors answer 409 EVENT_CURSOR_EXPIRED (04 §5).
func (h *SSEHandlers) Subscribe(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	ctx := c.Request.Context()
	var role string
	if err := h.Pool.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, p.UserID).Scan(&role); err != nil {
		// Membership revoked mid-stream also lands here via the periodic check.
		respond{}.error(c, apierrors.New(apierrors.NotFound, "project not found"))
		return
	}
	after := int64(0)
	if raw := c.Query("after"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || after < 0 {
			respond{}.error(c, apierrors.Fields("after", "invalid"))
			return
		}
	}
	if after == 0 {
		if raw := c.GetHeader("Last-Event-ID"); raw != "" {
			after, _ = strconv.ParseInt(raw, 10, 64)
		}
	}
	var cursor int64
	if err := h.Pool.QueryRow(ctx, `SELECT event_seq FROM projects WHERE id=$1`, projectID).Scan(&cursor); err != nil {
		respond{}.error(c, apierrors.New(apierrors.DependencyDown, "event store unavailable").WithRetryable(true))
		return
	}
	if cursor-after > 100000 {
		respond{}.error(c, apierrors.New(apierrors.CursorExpired, "cursor too old; re-bootstrap"))
		return
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}
	writeRow := func(row EventRow) {
		payload, _ := json.Marshal(row)
		fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", row.Seq, row.Type, payload)
		if flusher != nil {
			flusher.Flush()
		}
	}
	batch, err := h.fetch(ctx, projectID, after, 200)
	if err != nil {
		return
	}
	for _, row := range batch {
		writeRow(row)
		after = row.Seq
	}

	wake, cancel := h.Bus.subscribe(projectID.String())
	defer cancel()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	// In-process bus gives instant wake-ups; the 1s DB poll covers cross-
	// process writers (outbox delivery is best-effort wake, 04 §5).
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	membershipCheck := time.NewTicker(60 * time.Second)
	defer membershipCheck.Stop()
	drain := func() {
		batch, err := h.fetch(ctx, projectID, after, 200)
		if err != nil {
			return
		}
		for _, row := range batch {
			writeRow(row)
			after = row.Seq
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			drain()
		case <-wake:
			if !h.stillMember(ctx, projectID, p.UserID) {
				return
			}
			drain()
		case <-membershipCheck.C:
			if !h.stillMember(ctx, projectID, p.UserID) {
				return
			}
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat %d\n\n", time.Now().UnixMilli())
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func (h *SSEHandlers) stillMember(ctx context.Context, projectID, userID uuid.UUID) bool {
	var role string
	return h.Pool.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, userID).Scan(&role) == nil
}

// listNotifications returns the caller's notifications (06 §4).
func (h *SSEHandlers) listNotifications(c *gin.Context) {
	p := principalFrom(c)
	rows, err := h.Pool.Query(c.Request.Context(), `
		SELECT id, type, object_ref, read_at, created_at FROM notifications
		WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, p.UserID)
	if err != nil {
		respond{}.error(c, apierrors.New(apierrors.Internal, "notifications failed").Wrap(err))
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var (
			id        uuid.UUID
			typeName  string
			objectRef []byte
			readAt    *time.Time
			createdAt time.Time
		)
		if err := rows.Scan(&id, &typeName, &objectRef, &readAt, &createdAt); err != nil {
			respond{}.error(c, apierrors.New(apierrors.Internal, "scan failed").Wrap(err))
			return
		}
		ref := map[string]any{}
		_ = json.Unmarshal(objectRef, &ref)
		items = append(items, gin.H{"id": id, "type": typeName, "objectRef": ref,
			"readAt": readAt, "createdAt": createdAt})
	}
	respond{}.ok(c, respond{}.list(items, nil))
}

// markNotificationsRead flags read (no business state change, 04 §6).
func (h *SSEHandlers) markNotificationsRead(c *gin.Context) {
	p := principalFrom(c)
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	marked := 0
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		tag, err := h.Pool.Exec(c.Request.Context(), `
			UPDATE notifications SET read_at=COALESCE(read_at, now())
			WHERE id=$1 AND user_id=$2 AND read_at IS NULL`, id, p.UserID)
		if err == nil {
			marked += int(tag.RowsAffected())
		}
	}
	respond{}.ok(c, gin.H{"marked": marked})
}

// Register mounts the SSE + notification operations.
func (h *SSEHandlers) Register(spec *SpecRouter) {
	spec.Register("subscribeProjectEvents", withAuth(h.Subscribe))
	spec.Register("listNotifications", withAuth(h.listNotifications))
	spec.Register("markNotificationsRead", withAuth(h.markNotificationsRead))
}
