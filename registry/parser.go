package registry

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"agentd/control"
)

func Parse(raw []byte) ([]control.AgentCatalogEntry, error) {
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode registry: %w", err)
	}
	entries := make([]control.AgentCatalogEntry, 0, len(doc.Agents))
	for _, agent := range doc.Agents {
		if err := validateAgent(agent); err != nil {
			return nil, err
		}
		entries = append(entries, control.AgentCatalogEntry{
			ID:           agent.ID,
			Version:      agent.Version,
			Name:         agent.Name,
			Description:  agent.Description,
			Source:       control.AgentSourceRegistry,
			Protocol:     agent.Protocol,
			Distribution: agent.Distribution,
			DefaultEnv:   agent.DefaultEnv,
			Metadata:     agent.Metadata,
			Platforms:    agent.Platforms,
			Private:      agent.Private,
			UpdatedAt:    time.Now().UTC(),
		})
	}
	return entries, nil
}

func Read(path string) ([]control.AgentCatalogEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

func validateAgent(agent Agent) error {
	if strings.TrimSpace(agent.ID) == "" {
		return fmt.Errorf("registry agent missing id")
	}
	if strings.TrimSpace(agent.Version) == "" {
		return fmt.Errorf("registry agent %s missing version", agent.ID)
	}
	switch agent.Distribution.Type {
	case control.DistributionBinary:
		if agent.Distribution.Binary == nil {
			return fmt.Errorf("registry agent %s binary distribution missing payload", agent.ID)
		}
		if _, err := url.Parse(agent.Distribution.Binary.URL); err != nil {
			return fmt.Errorf("registry agent %s invalid binary url: %w", agent.ID, err)
		}
	case control.DistributionNPX:
		if agent.Distribution.NPX == nil || agent.Distribution.NPX.Package == "" {
			return fmt.Errorf("registry agent %s npx distribution missing package", agent.ID)
		}
	case control.DistributionUVX:
		if agent.Distribution.UVX == nil || agent.Distribution.UVX.Package == "" {
			return fmt.Errorf("registry agent %s uvx distribution missing package", agent.ID)
		}
	default:
		return fmt.Errorf("registry agent %s unsupported distribution %q", agent.ID, agent.Distribution.Type)
	}
	return nil
}
