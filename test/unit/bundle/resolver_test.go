package bundle_test

import (
	"testing"

	"agentd/bundle"
	"agentd/control"
)

func TestResolveBundleBinaryAndNPX(t *testing.T) {
	resolver := bundle.Resolver{}
	binary, err := resolver.Resolve(control.AgentCatalogEntry{
		ID:      "bin",
		Version: "1.0.0",
		Distribution: control.DistributionSpec{
			Type:   control.DistributionBinary,
			Binary: &control.BinaryDistribution{URL: "https://example.invalid/bin", Executable: "/bin/agent"},
		},
	}, "linux-arm64")
	if err != nil {
		t.Fatalf("resolve binary: %v", err)
	}
	if !binary.Materialized || binary.Entrypoint[0] != "/bin/agent" {
		t.Fatalf("unexpected binary bundle: %+v", binary)
	}
	npx, err := resolver.Resolve(control.AgentCatalogEntry{
		ID:      "npx",
		Version: "1.0.0",
		Distribution: control.DistributionSpec{
			Type: control.DistributionNPX,
			NPX:  &control.PackageDistribution{Package: "pkg", Version: "2.0.0"},
		},
	}, "linux-arm64")
	if err != nil {
		t.Fatalf("resolve npx: %v", err)
	}
	if npx.Materialized || npx.Entrypoint[0] != "npx" {
		t.Fatalf("unexpected npx bundle: %+v", npx)
	}
}
