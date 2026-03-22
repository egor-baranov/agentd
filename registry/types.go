package registry

import "agentd/control"

type Document struct {
	Agents []Agent `json:"agents"`
}

type Agent struct {
	ID           string                   `json:"id"`
	Version      string                   `json:"version"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description,omitempty"`
	Platforms    []string                 `json:"platforms,omitempty"`
	Private      bool                     `json:"private,omitempty"`
	Metadata     map[string]string        `json:"metadata,omitempty"`
	DefaultEnv   map[string]string        `json:"default_env,omitempty"`
	Protocol     control.ACPProtocol      `json:"protocol"`
	Distribution control.DistributionSpec `json:"distribution"`
}
