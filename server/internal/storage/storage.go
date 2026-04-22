package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Storage defines the interface for file attachment operations.
type Storage interface {
	// Upload saves data under the given key and returns the resulting URL.
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
	// Delete removes the file at the given key.
	Delete(ctx context.Context, key string) error
}

// LocalStorage stores files on the local filesystem.
type LocalStorage struct {
	dir  string
	base string
}

// NewLocalStorage creates a LocalStorage that writes to `dir` and serves at `baseURL`.
func NewLocalStorage(dir, baseURL string) *LocalStorage {
	return &LocalStorage{dir: dir, base: baseURL}
}

func (s *LocalStorage) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	if err := os.MkdirAll(filepath.Join(s.dir, filepath.Dir(key)), 0o755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}
	path := filepath.Join(s.dir, key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(s.base, "/"), key), nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	return os.Remove(filepath.Join(s.dir, key))
}
