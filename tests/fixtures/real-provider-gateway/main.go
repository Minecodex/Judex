// A namespace-scoped acceptance proxy: one controlled 429, then unchanged real
// provider traffic. It never logs prompts, credentials or per-request IDs.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type prefixMetrics struct {
	Requests, Comparisons, CommonBytes, MinCommonBytes, MaxCommonBytes          int
	CompleteMessageMatches, PreviousMessagesRetained, ModelChanges, ToolChanges int
}

type gateway struct {
	holdNext, held                                       bool
	holds, holdTimeouts                                  int
	release                                              chan struct{}
	mu                                                   sync.Mutex
	proxy                                                *httputil.ReverseProxy
	requests, controlled429, highEffort, transportErrors int
	statuses                                             map[string]int
	prefix                                               prefixMetrics
	previous                                             [][]byte
	previousModel                                        string
	previousTools                                        []byte
}

func newGateway(target *url.URL) *gateway {
	g := &gateway{statuses: map[string]int{}, proxy: httputil.NewSingleHostReverseProxy(target)}
	direct := g.proxy.Director
	g.proxy.Director = func(r *http.Request) { direct(r); r.Host = target.Host }
	g.proxy.ModifyResponse = func(r *http.Response) error {
		g.mu.Lock()
		g.statuses[strconv.Itoa(r.StatusCode)]++
		var release chan struct{}
		if g.holdNext && r.StatusCode == 200 {
			g.holdNext, g.held = false, true
			g.holds++
			release = make(chan struct{})
			g.release = release
		}
		g.mu.Unlock()
		if release != nil {
			ctx := context.Background()
			if r.Request != nil {
				ctx = r.Request.Context()
			}
			timer := time.NewTimer(60 * time.Second)
			defer timer.Stop()
			var err error
			select {
			case <-release:
			case <-ctx.Done():
				err = ctx.Err()
			case <-timer.C:
				err = context.DeadlineExceeded
				g.mu.Lock()
				g.holdTimeouts++
				g.mu.Unlock()
			}
			g.mu.Lock()
			g.held = false
			if g.release == release {
				g.release = nil
			}
			g.mu.Unlock()
			return err
		}
		return nil
	}
	g.proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		g.mu.Lock()
		g.transportErrors++
		g.mu.Unlock()
		http.Error(w, "acceptance proxy upstream transport failed", http.StatusBadGateway)
	}
	return g
}

func (g *gateway) observe(body []byte) (bool, bool) {
	var request struct {
		Model    string `json:"model"`
		Messages []any  `json:"messages"`
		Tools    any    `json:"tools"`
		Effort   string `json:"reasoning_effort"`
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if d.Decode(&request) != nil || len(request.Messages) == 0 {
		return false, false
	}
	if request.Effort != "high" {
		return false, false
	}
	current := make([][]byte, len(request.Messages))
	for i, message := range request.Messages {
		current[i], _ = json.Marshal(message)
	}
	tools, _ := json.Marshal(request.Tools)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests++
	g.prefix.Requests++
	if request.Effort == "high" {
		g.highEffort++
	}
	if g.previous != nil {
		g.prefix.Comparisons++
		old, next := bytes.Join(g.previous, []byte{','}), bytes.Join(current, []byte{','})
		common := 0
		for common < len(old) && common < len(next) && old[common] == next[common] {
			common++
		}
		g.prefix.CommonBytes += common
		if g.prefix.Comparisons == 1 || common < g.prefix.MinCommonBytes {
			g.prefix.MinCommonBytes = common
		}
		if common > g.prefix.MaxCommonBytes {
			g.prefix.MaxCommonBytes = common
		}
		matched := 0
		for matched < len(g.previous) && matched < len(current) && bytes.Equal(g.previous[matched], current[matched]) {
			matched++
		}
		g.prefix.CompleteMessageMatches += matched
		if matched == len(g.previous) {
			g.prefix.PreviousMessagesRetained++
		}
		if request.Model != g.previousModel {
			g.prefix.ModelChanges++
		}
		if !bytes.Equal(tools, g.previousTools) {
			g.prefix.ToolChanges++
		}
	}
	g.previous, g.previousTools, g.previousModel = current, tools, request.Model
	first := g.requests == 1
	if first {
		g.controlled429++
	}
	return first, request.Effort == "high"
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" && r.URL.Path == "/control/hold-next" {
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.holdNext || g.held {
			http.Error(w, "acceptance hold already armed", 409)
			return
		}
		g.holdNext = true
		w.WriteHeader(204)
		return
	}
	if r.Method == "POST" && r.URL.Path == "/control/release" {
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.release != nil {
			close(g.release)
			g.release = nil
		}
		w.WriteHeader(204)
		return
	}
	if r.URL.Path == "/healthz" {
		w.WriteHeader(200)
		return
	}
	if r.URL.Path == "/stats" {
		g.mu.Lock()
		defer g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"scope": "local canonical Go JSON messages, one controlled reject included; no provider cache hits or billing measured", "requests": g.requests, "controlled429": g.controlled429, "highEffortRequests": g.highEffort, "upstreamStatuses": g.statuses, "transportErrors": g.transportErrors, "messagePrefix": g.prefix, "responseHeld": g.held, "holds": g.holds, "holdTimeouts": g.holdTimeouts})
		return
	}
	if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
	_ = r.Body.Close()
	if err != nil || len(body) > 8<<20 {
		http.Error(w, "bounded acceptance request required", 400)
		return
	}
	first, high := g.observe(body)
	if !high {
		http.Error(w, "acceptance requires unchanged high reasoning effort", 400)
		return
	}
	if first {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(429)
		_, _ = io.WriteString(w, `{"error":{"type":"rate_limit_error","message":"controlled acceptance rate limit; not provider quota exhaustion"}}`)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	g.proxy.ServeHTTP(w, r)
}

func main() {
	target, _ := url.Parse("https://api.apimart.ai")
	server := &http.Server{Addr: ":8090", Handler: newGateway(target)}
	if server.ListenAndServe() != http.ErrServerClosed {
		panic("acceptance proxy stopped")
	}
}
