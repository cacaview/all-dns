package notifier

import (
	"context"
	"net"
	"testing"
)

func TestEmailNotifier_Enabled(t *testing.T) {
	n := NewEmailNotifier("smtp.example.com", 587, "user", "pass", "DNS Hub <noreply@example.com>")
	if !n.Enabled() {
		t.Error("expected enabled with valid host/port")
	}
}

func TestEmailNotifier_NotEnabled_EmptyHost(t *testing.T) {
	n := NewEmailNotifier("", 587, "user", "pass", "from@example.com")
	if n.Enabled() {
		t.Error("expected disabled with empty host")
	}
}

func TestEmailNotifier_NotEnabled_ZeroPort(t *testing.T) {
	n := NewEmailNotifier("smtp.example.com", 0, "user", "pass", "from@example.com")
	if n.Enabled() {
		t.Error("expected disabled with zero port")
	}
}

func TestEmailNotifier_Notify_ConnectionRefused(t *testing.T) {
	n := NewEmailNotifier("localhost", 59999, "user", "pass", "from@example.com")
	err := n.Notify(context.Background(), map[string]any{
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})
	if err == nil {
		t.Error("expected error when connection is refused")
	}
}

func TestEmailNotifier_Notify_NotEnabled(t *testing.T) {
	n := NewEmailNotifier("", 587, "user", "pass", "from@example.com")
	err := n.Notify(context.Background(), map[string]any{"to": []string{"to@example.com"}})
	if err != nil {
		t.Errorf("expected nil error when disabled, got %v", err)
	}
}

func TestEmailNotifier_Notify_MissingTo(t *testing.T) {
	n := NewEmailNotifier("smtp.example.com", 587, "user", "pass", "from@example.com")
	err := n.Notify(context.Background(), map[string]any{"subject": "Test", "body": "Hello"})
	if err == nil {
		t.Error("expected error for missing 'to' field")
	}
}

func TestEmailNotifier_Notify_EmptyTo(t *testing.T) {
	n := NewEmailNotifier("smtp.example.com", 587, "user", "pass", "from@example.com")
	err := n.Notify(context.Background(), map[string]any{"to": []string{}, "subject": "Test", "body": "Hello"})
	if err == nil {
		t.Error("expected error for empty 'to' list")
	}
}

func TestEmailNotifier_Notify_ContextCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	// Don't accept connections — client will block trying to connect, triggering context cancel
	n := NewEmailNotifier("localhost", listener.Addr().(*net.TCPAddr).Port, "user", "pass", "from@example.com")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = n.Notify(ctx, map[string]any{
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})
	if err == nil {
		t.Error("expected error for canceled context")
	}
}
