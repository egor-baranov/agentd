package rpc

import (
	"agentd/control"
	"agentd/session"
)

type Empty struct{}

type IDRequest struct {
	ID string `json:"id"`
}

type AgentResponse struct {
	Agent control.AgentInstance `json:"agent"`
}

type SessionResponse struct {
	Session control.Session `json:"session"`
}

type ContainerResponse struct {
	Container control.Container `json:"container"`
}

type ContainerSnapshotResponse struct {
	Snapshot control.ContainerSnapshot `json:"snapshot"`
}

type PromptStreamItem struct {
	Event session.Event `json:"event"`
	Run   control.Run   `json:"run"`
	Final bool          `json:"final"`
	Error string        `json:"error,omitempty"`
}

type ListAgentsResponse struct {
	Agents []control.AgentInstance `json:"agents"`
}

type ListSessionsRequest struct {
	ContainerID string `json:"container_id,omitempty"`
}

type ListSessionsResponse struct {
	Sessions []control.Session `json:"sessions"`
}
