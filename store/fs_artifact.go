package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FSArtifactStore struct {
	root string
}

func NewFSArtifactStore(root string) *FSArtifactStore {
	return &FSArtifactStore{root: root}
}

func (s *FSArtifactStore) Put(_ context.Context, key, _ string, body io.Reader) error {
	path := filepath.Join(s.root, cleanKey(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, body)
	return err
}

func (s *FSArtifactStore) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	path := filepath.Join(s.root, cleanKey(key))
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", path, err)
	}
	return f, "application/octet-stream", nil
}

func cleanKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	return filepath.Clean(key)
}
