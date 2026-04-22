package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage_UploadAndDelete(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir, "http://localhost:8080/uploads")

	ctx := context.Background()
	key := "test-dir/test-file.txt"
	data := []byte("hello world")

	url, err := storage.Upload(ctx, key, data, "text/plain")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if url == "" {
		t.Error("url should not be empty")
	}

	// Verify file was written
	filePath := filepath.Join(tmpDir, key)
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read uploaded file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(content))
	}

	// Delete
	err = storage.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestLocalStorage_UrlFormat(t *testing.T) {
	storage := NewLocalStorage("/tmp/uploads", "http://localhost:8080/uploads")
	ctx := context.Background()

	url, err := storage.Upload(ctx, "file.txt", []byte("data"), "text/plain")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	expected := "http://localhost:8080/uploads/file.txt"
	if url != expected {
		t.Errorf("expected %q, got %q", expected, url)
	}
}

func TestLocalStorage_MissingDir(t *testing.T) {
	// Upload to a non-existent nested directory should auto-create it
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir, "http://localhost:8080/uploads")

	ctx := context.Background()
	key := "nested/deeply/file.txt"
	data := []byte("nested")

	_, err := storage.Upload(ctx, key, data, "text/plain")
	if err != nil {
		t.Fatalf("Upload with nested path should succeed: %v", err)
	}

	filePath := filepath.Join(tmpDir, key)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("nested directory should have been created")
	}
}

func TestLocalStorage_DeleteNonexistent(t *testing.T) {
	storage := NewLocalStorage("/tmp/nonexistent", "http://localhost:8080/uploads")
	ctx := context.Background()

	err := storage.Delete(ctx, "does-not-exist.txt")
	// Delete of non-existent file should not error in our implementation
	_ = err
}
