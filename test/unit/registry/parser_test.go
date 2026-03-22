package registry_test

import (
	"testing"

	"agentd/registry"
)

func TestParseRegistry(t *testing.T) {
	entries, err := registry.Parse([]byte(`{"agents":[{"id":"a1","version":"1.0.0","name":"Agent","protocol":{"auth_modes":["agent"]},"distribution":{"type":"npx","npx":{"package":"pkg","version":"1.0.0"}}}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "a1" || entries[0].Distribution.NPX.Package != "pkg" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}
