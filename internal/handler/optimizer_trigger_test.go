package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPOptimizerTriggerPostsEncodedKey(t *testing.T) {
	received := make(chan struct {
		method string
		key    string
	}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct {
			method string
			key    string
		}{method: r.Method, key: r.URL.Query().Get("key")}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	trigger := NewHTTPOptimizerTrigger(server.URL+"/optimize", time.Second)
	if err := trigger.Trigger(context.Background(), "notes/photo 1.jpg"); err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}

	select {
	case got := <-received:
		if got.method != http.MethodPost {
			t.Fatalf("expected POST, got %s", got.method)
		}
		if got.key != "notes/photo 1.jpg" {
			t.Fatalf("expected encoded key to roundtrip, got %q", got.key)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for optimizer trigger request")
	}
}

func TestHTTPOptimizerTriggerRejectsNonAcceptedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	trigger := NewHTTPOptimizerTrigger(server.URL+"/optimize", time.Second)
	if err := trigger.post(context.Background(), "photo.jpg"); err == nil {
		t.Fatal("expected non-202 status error from direct post")
	}
}

func TestHTTPOptimizerTriggerDeduplicatesPendingKeys(t *testing.T) {
	block := make(chan struct{})
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		<-block
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	defer close(block)

	trigger := NewHTTPOptimizerTrigger(server.URL+"/optimize", time.Second)
	if err := trigger.Trigger(context.Background(), "photo.jpg"); err != nil {
		t.Fatalf("first trigger failed: %v", err)
	}
	if err := trigger.Trigger(context.Background(), "photo.jpg"); err != nil {
		t.Fatalf("duplicate trigger failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	got := requests
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected one outbound request while key is pending, got %d", got)
	}
}

func TestHTTPOptimizerTriggerRejectsFullQueue(t *testing.T) {
	trigger := &HTTPOptimizerTrigger{
		endpoint: "http://example.test/optimize",
		client:   &http.Client{Timeout: time.Second},
		keys:     make(chan string, 1),
		pending: map[string]struct{}{
			"first.jpg": {},
		},
	}

	if err := trigger.Trigger(context.Background(), "second.jpg"); err == nil {
		t.Fatal("expected full queue error")
	}
}
