package control_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"agentd/control"
)

func TestConfigLoaderConcurrentSnapshots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"http_addr":":9000"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loader := control.NewConfigLoader(path)
	if _, err := loader.Load(); err != nil {
		t.Fatal(err)
	}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = loader.Snapshot()
			}
		}()
	}
	if err := os.WriteFile(path, []byte(`{"http_addr":":9001"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loader.Load(); err != nil {
		t.Fatal(err)
	}
	cancel()
	wg.Wait()
	if got := loader.Snapshot().HTTPAddr; got != ":9001" {
		t.Fatalf("expected updated snapshot, got %q", got)
	}
}
