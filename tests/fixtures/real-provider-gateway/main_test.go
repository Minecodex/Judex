package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestControlledRejectThenExactForwardingAndPrivateAggregate(t *testing.T) {
	body := []byte(`{"model":"glm-5.3-flash","reasoning_effort":"high","messages":[{"role":"system","content":"private-test-sentence 中文"},{"role":"user","content":"read fixed input"}],"tools":[{"type":"function","function":{"name":"read_workflow"}}]}`)
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		got, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer unit-test-secret" || !bytes.Equal(got, body) {
			t.Error("proxy changed path, credential or request bytes")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	proxy := httptest.NewServer(newGateway(target))
	defer proxy.Close()
	for _, status := range []int{429, 200} {
		r, _ := http.NewRequest("POST", proxy.URL+"/v1/chat/completions", bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer unit-test-secret")
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("status %d, expected %d", response.StatusCode, status)
		}
		if status == 200 && !strings.Contains(string(got), "data: [DONE]") {
			t.Fatal("stream changed")
		}
	}
	response, err := http.Get(proxy.URL + "/stats")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if strings.Contains(string(raw), "unit-test-secret") || strings.Contains(string(raw), "private-test-sentence") {
		t.Fatal("private content entered aggregate")
	}
	var stats struct {
		Requests, Controlled429, HighEffortRequests int
		MessagePrefix                               prefixMetrics
	}
	if json.Unmarshal(raw, &stats) != nil {
		t.Fatal("stats invalid")
	}
	if upstreamCalls != 1 || stats.Requests != 2 || stats.Controlled429 != 1 || stats.HighEffortRequests != 2 || stats.MessagePrefix.PreviousMessagesRetained != 1 || stats.MessagePrefix.CompleteMessageMatches != 2 {
		t.Fatalf("wrong transport/prefix aggregate: %s", raw)
	}
}

func TestOperatorHoldPreservesRealStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	proxy := httptest.NewServer(newGateway(target))
	defer proxy.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	body := `{"model":"fixture","reasoning_effort":"high","messages":[{"role":"user","content":"timing input"}]}`
	post := func(path, content string) (*http.Response, error) {
		return client.Post(proxy.URL+path, "application/json", strings.NewReader(content))
	}
	first, err := post("/v1/chat/completions", body)
	if err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	armed, err := post("/control/hold-next", "")
	if err != nil || armed.StatusCode != 204 {
		t.Fatalf("arm: %v", err)
	}
	armed.Body.Close()
	completed := make(chan string, 1)
	go func() {
		response, err := post("/v1/chat/completions", body)
		if err != nil {
			completed <- err.Error()
			return
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		completed <- string(raw)
	}()
	deadline := time.Now().Add(3 * time.Second)
	held := false
	for time.Now().Before(deadline) {
		response, err := client.Get(proxy.URL + "/stats")
		if err != nil {
			t.Fatal(err)
		}
		var stats struct {
			Held bool `json:"responseHeld"`
		}
		_ = json.NewDecoder(response.Body).Decode(&stats)
		response.Body.Close()
		if stats.Held {
			held = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !held {
		t.Fatal("response was not held")
	}
	select {
	case <-completed:
		t.Fatal("response escaped the operator checkpoint")
	default:
	}
	released, err := post("/control/release", "")
	if err != nil || released.StatusCode != 204 {
		t.Fatalf("release: %v", err)
	}
	released.Body.Close()
	select {
	case raw := <-completed:
		if raw != "data: {\"choices\":[]}\n\ndata: [DONE]\n\n" {
			t.Fatalf("held stream changed: %s", raw)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("release did not converge")
	}
}
