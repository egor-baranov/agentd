package control

type CreateContainerRequest struct {
	ID       string            `json:"id,omitempty"`
	Profile  ContainerProfile  `json:"profile"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Capacity int               `json:"capacity,omitempty"`
	Pinned   bool              `json:"pinned,omitempty"`
}

type InstallAgentRequest struct {
	ContainerID string        `json:"container_id"`
	Plan        ExecutionPlan `json:"plan"`
}

type CreateSessionRequest struct {
	SessionID       string        `json:"session_id,omitempty"`
	ContainerID     string        `json:"container_id"`
	AgentInstanceID string        `json:"agent_instance_id,omitempty"`
	Plan            ExecutionPlan `json:"plan"`
}

type LoadSessionRequest struct {
	SessionID       string      `json:"session_id,omitempty"`
	ContainerID     string      `json:"container_id"`
	AgentInstanceID string      `json:"agent_instance_id"`
	ResumeID        string      `json:"resume_id"`
	Spec            SessionSpec `json:"spec"`
}

type PromptRequest struct {
	SessionID string       `json:"session_id"`
	RunID     string       `json:"run_id,omitempty"`
	Input     SessionInput `json:"input"`
}

type CancelRequest struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
}

type ContainerSnapshot struct {
	Container Container       `json:"container"`
	Agents    []AgentInstance `json:"agents,omitempty"`
	Sessions  []Session       `json:"sessions,omitempty"`
}

type SessionDetail struct {
	Session Session `json:"session"`
	Runs    []Run   `json:"runs,omitempty"`
}
