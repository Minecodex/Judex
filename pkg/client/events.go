package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type EventPage struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor int64             `json:"nextCursor"`
}

func (c *Client) Events(ctx context.Context, project string, after int64, limit int) (EventPage, error) {
	out := EventPage{Items: []json.RawMessage{}, NextCursor: after}
	req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"/api/v1/projects/"+project+"/events?after="+strconv.FormatInt(after, 10), nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", strconv.FormatInt(after, 10))
	if err := c.authorize(ctx, req); err != nil {
		return out, err
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		return out, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return out, &CLIError{Status: response.StatusCode, Code: "HTTP_ERROR", Message: response.Status}
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var id int64
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "id:"):
			id, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "id:")), 10, 64)
		case strings.HasPrefix(line, "data:"):
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case line == "" && data.Len() > 0:
			raw := json.RawMessage(data.String())
			data.Reset()
			if !json.Valid(raw) {
				return out, fmt.Errorf("invalid event JSON")
			}
			if id <= out.NextCursor {
				continue
			}
			out.Items = append(out.Items, raw)
			out.NextCursor = id
			if len(out.Items) >= limit {
				return out, nil
			}
		}
	}
	if ctx.Err() != nil {
		return out, nil
	}
	return out, scanner.Err()
}
