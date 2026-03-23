package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentd/api"
	"agentd/control"
	"agentd/observer"
	"agentd/store"
)

func TestServerHealthAndRegistryList(t *testing.T) {
	mem := store.NewMemoryStore()
	if err := mem.UpsertCatalog(context.Background(), []control.AgentCatalogEntry{{
		ID:      "echo-local",
		Version: "1.0.0",
		Name:    "Echo Local",
	}}); err != nil {
		t.Fatalf("seed registry: %v", err)
	}

	srv := httptest.NewServer((&api.Server{
		Metrics:  observer.NewMetrics(),
		Registry: mem,
	}).Handler())
	defer srv.Close()

	t.Run("healthz", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + "/healthz")
		if err != nil {
			t.Fatalf("get healthz: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected healthz status: %d", resp.StatusCode)
		}
	})

	t.Run("readyz", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + "/readyz")
		if err != nil {
			t.Fatalf("get readyz: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected readyz status: %d", resp.StatusCode)
		}
	})

	t.Run("registry list", func(t *testing.T) {
		client := srv.Client()
		client.Timeout = 2 * time.Second
		resp, err := client.Get(srv.URL + "/v1/registry/agents")
		if err != nil {
			t.Fatalf("get registry list: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected registry status: %d", resp.StatusCode)
		}
		var payload struct {
			Agents []control.AgentCatalogEntry `json:"agents"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode registry list: %v", err)
		}
		if len(payload.Agents) != 1 || payload.Agents[0].ID != "echo-local" {
			t.Fatalf("unexpected registry payload: %+v", payload.Agents)
		}
	})
}
