package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"agentd/control"
)

type Resolver struct{}

func (Resolver) Resolve(entry control.AgentCatalogEntry, platform string) (control.Bundle, error) {
	if platform == "" {
		platform = normalizePlatform(runtime.GOOS + "-" + runtime.GOARCH)
	}
	if len(entry.Platforms) > 0 {
		matched := false
		for _, p := range entry.Platforms {
			if normalizePlatform(p) == platform {
				matched = true
				break
			}
		}
		if !matched {
			return control.Bundle{}, fmt.Errorf("%w: platform %s for agent %s", control.ErrUnsupportedBundle, platform, entry.ID)
		}
	}
	bundle := control.Bundle{
		AgentID:   entry.ID,
		Version:   entry.Version,
		Platform:  platform,
		Manifest:  entry.Distribution,
		CreatedAt: time.Now().UTC(),
		Env:       map[string]string{},
	}
	switch entry.Distribution.Type {
	case control.DistributionBinary:
		spec := entry.Distribution.Binary
		bundle.Mode = control.BundleModeMaterialized
		bundle.Materialized = true
		bundle.Entrypoint = append([]string{spec.Executable}, spec.Args...)
		bundle.Env["AGENTD_BINARY_URL"] = spec.URL
		if spec.SHA256 != "" {
			bundle.Env["AGENTD_BINARY_SHA256"] = spec.SHA256
		}
	case control.DistributionNPX:
		spec := entry.Distribution.NPX
		bundle.Mode = control.BundleModeLazy
		bundle.Entrypoint = append([]string{"npx", "-y", packageRef(spec.Package, spec.Version)}, spec.Args...)
	case control.DistributionUVX:
		spec := entry.Distribution.UVX
		bundle.Mode = control.BundleModeLazy
		bundle.Entrypoint = append([]string{"uvx", packageRef(spec.Package, spec.Version)}, spec.Args...)
	default:
		return control.Bundle{}, fmt.Errorf("%w: %s", control.ErrUnsupportedBundle, entry.Distribution.Type)
	}
	payload, err := json.Marshal(struct {
		AgentID    string                   `json:"agent_id"`
		Version    string                   `json:"version"`
		Platform   string                   `json:"platform"`
		Entrypoint []string                 `json:"entrypoint"`
		Manifest   control.DistributionSpec `json:"manifest"`
	}{entry.ID, entry.Version, platform, bundle.Entrypoint, entry.Distribution})
	if err != nil {
		return control.Bundle{}, err
	}
	sum := sha256.Sum256(payload)
	bundle.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return bundle, nil
}

func normalizePlatform(platform string) string {
	platform = strings.ToLower(platform)
	replacer := strings.NewReplacer("/", "-", "_", "-")
	return replacer.Replace(platform)
}

func packageRef(pkg, version string) string {
	if version == "" {
		return pkg
	}
	return pkg + "@" + version
}
