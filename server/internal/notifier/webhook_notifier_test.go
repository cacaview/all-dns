package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebhookNotifier_Enabled(t *testing.T) {
	n := NewWebhookNotifier("https://example.com/webhook")
	if !n.Enabled() {
		t.Error("expected enabled when URL is set")
	}
}

func TestWebhookNotifier_NotEnabled_EmptyURL(t *testing.T) {
	n := NewWebhookNotifier("")
	if n.Enabled() {
		t.Error("expected disabled when URL is empty")
	}
}

func TestWebhookNotifier_NotEnabled_WhitespaceURL(t *testing.T) {
	n := NewWebhookNotifier("   ")
	if n.Enabled() {
		t.Error("expected disabled when URL is only whitespace")
	}
}

func TestWebhookNotifier_Notify_Success(t *testing.T) {
	var receivedBody map[string]any
	var called int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(server.URL)
	err := n.Notify(context.Background(), map[string]any{"type": "test", "data": 123})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected server called 1 time, got %d", called)
	}
	if receivedBody["type"] != "test" {
		t.Errorf("expected type='test', got %v", receivedBody["type"])
	}
	if receivedBody["data"] != float64(123) {
		t.Errorf("expected data=123, got %v", receivedBody["data"])
	}
}

func TestWebhookNotifier_Notify_NotEnabled(t *testing.T) {
	n := NewWebhookNotifier("")
	err := n.Notify(context.Background(), map[string]any{"type": "test"})
	if err != nil {
		t.Errorf("expected nil error when disabled, got %v", err)
	}
}

func TestWebhookNotifier_Notify_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := NewWebhookNotifier(server.URL)
	err := n.Notify(context.Background(), map[string]any{"type": "test"})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestWebhookNotifier_Notify_Server400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	n := NewWebhookNotifier(server.URL)
	err := n.Notify(context.Background(), map[string]any{"type": "test"})
	if err == nil {
		t.Error("expected error for 400 response")
	}
}

func TestWebhookNotifier_Notify_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create notifier with short timeout via direct struct (not exported, use NewWebhookNotifier with URL that causes timeout)
	// Instead test that the default 10s timeout is respected
	n := NewWebhookNotifier(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	err := n.Notify(ctx, map[string]any{"type": "test"})
	if err == nil {
		t.Error("expected error for context deadline exceeded")
	}
}

func TestWebhookNotifier_Notify_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	err := n.Notify(ctx, map[string]any{"type": "test"})
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestWebhookNotifier_NewWebhookNotifier(t *testing.T) {
	n := NewWebhookNotifier("  https://example.com/wh  ")
	if !n.Enabled() {
		t.Error("expected enabled, URL should be trimmed")
	}
}

func TestWebhookNotifier_Notify_Retryable_ConnectionRefused(t *testing.T) {
	// A server that refuses connection - this tests the error path
	n := NewWebhookNotifier("http://localhost:1/webhook")
	err := n.Notify(context.Background(), map[string]any{"type": "test"})
	if err == nil {
		t.Error("expected error for connection refused")
	}
}
