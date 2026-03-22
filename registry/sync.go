package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"agentd/control"
	"agentd/store"
)

type Syncer struct {
	Store      store.RegistryStore
	HTTPClient *http.Client
}

func (s Syncer) Sync(ctx context.Context, source string) ([]control.AgentCatalogEntry, error) {
	raw, err := s.readSource(ctx, source)
	if err != nil {
		return nil, err
	}
	entries, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	if err := s.Store.UpsertCatalog(ctx, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s Syncer) readSource(ctx context.Context, source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		client := s.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("registry fetch %s: %s", source, resp.Status)
		}
		return io.ReadAll(resp.Body)
	}
	return os.ReadFile(source)
}
