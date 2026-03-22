package acp

import "agentd/control"

type InitializeRequest struct {
	ClientName    string `json:"client_name"`
	ClientVersion string `json:"client_version"`
}

type InitializeResponse struct {
	ProtocolVersion string             `json:"protocol_version"`
	Capabilities    []string           `json:"capabilities,omitempty"`
	AuthMethods     []control.AuthMode `json:"auth_methods,omitempty"`
}

type SessionResponse struct {
	SessionID string         `json:"session_id"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type SessionNewParams struct {
	SessionID  string                  `json:"session_id,omitempty"`
	Metadata   map[string]any          `json:"metadata,omitempty"`
	MCPServers []control.MCPServerSpec `json:"mcp_servers,omitempty"`
}

type SessionLoadParams struct {
	SessionID string `json:"session_id"`
}

type PromptParams struct {
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id"`
	Prompt    string         `json:"prompt,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type CancelParams struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
}

type SessionListResponse struct {
	SessionIDs []string `json:"session_ids,omitempty"`
}

type StreamEvent struct {
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id,omitempty"`
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Final     bool           `json:"final,omitempty"`
	Error     string         `json:"error,omitempty"`
}
