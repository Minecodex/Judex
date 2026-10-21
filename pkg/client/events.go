package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type StreamEvent struct {
	Seq  int64           `json:"seq"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// FollowEvents reconnects with the last delivered sequence, refreshing bearer
// credentials through the same token source as ordinary commands.
func (c *Client) FollowEvents(ctx context.Context, conversation string, after int64, limit int, emit func(StreamEvent) error) error {
	if err := c.CheckProtocol(ctx); err != nil {
		return err
	}
	received := 0
	for {
		req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"/api/v1/conversations/"+conversation+"/events?after="+strconv.FormatInt(after, 10), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Last-Event-ID", strconv.FormatInt(after, 10))
		if err = c.authorize(ctx, req); err != nil {
			return err
		}
		transport := *c.HTTP
		transport.Timeout = 0
		response, err := transport.Do(req)
		if err == nil {
			if response.StatusCode >= 400 {
				response.Body.Close()
				return &CLIError{Status: response.StatusCode, Code: "HTTP_ERROR", Message: response.Status}
			}
			scanner := bufio.NewScanner(response.Body)
			scanner.Buffer(make([]byte, 4096), 1<<20)
			id := after
			kind := "message"
			var data strings.Builder
			for scanner.Scan() {
				line := scanner.Text()
				switch {
				case strings.HasPrefix(line, "id:"):
					id, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "id:")), 10, 64)
				case strings.HasPrefix(line, "event:"):
					kind = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				case strings.HasPrefix(line, "data:"):
					if data.Len() > 0 {
						data.WriteByte('\n')
					}
					data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				case line == "" && data.Len() > 0:
					raw := json.RawMessage(data.String())
					data.Reset()
					if !json.Valid(raw) {
						response.Body.Close()
						return fmt.Errorf("invalid SSE JSON")
					}
					if id <= after {
						continue
					}
					if err = emit(StreamEvent{Seq: id, Kind: kind, Data: raw}); err != nil {
						response.Body.Close()
						return err
					}
					after = id
					received++
					if limit > 0 && received >= limit {
						response.Body.Close()
						return nil
					}
				}
			}
			response.Body.Close()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
	}
}
