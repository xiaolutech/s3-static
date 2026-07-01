package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPOptimizerTriggerPostsEncodedKey(t *testing.T) {
	var gotMethod string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.Query().Get("key")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	trigger := NewHTTPOptimizerTrigger(server.URL+"/optimize", time.Second)
	if err := trigger.Trigger(context.Background(), "notes/photo 1.jpg"); err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotQuery != "notes/photo 1.jpg" {
		t.Fatalf("expected encoded key to roundtrip, got %q", gotQuery)
	}
}

func TestHTTPOptimizerTriggerRejectsNonAcceptedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	trigger := NewHTTPOptimizerTrigger(server.URL+"/optimize", time.Second)
	if err := trigger.Trigger(context.Background(), "photo.jpg"); err == nil {
		t.Fatal("expected non-202 status error")
	}
}
