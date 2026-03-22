package control

import "time"

type DistributionType string

const (
	DistributionBinary DistributionType = "binary"
	DistributionNPX    DistributionType = "npx"
	DistributionUVX    DistributionType = "uvx"
)

type AgentSource string

const (
	AgentSourceRegistry AgentSource = "registry"
	AgentSourceManual   AgentSource = "manual"
)

type AuthMode string

const (
	AuthModeAgent    AuthMode = "agent"
	AuthModeTerminal AuthMode = "terminal"
	AuthModeNone     AuthMode = "none"
)

type BundleMode string

const (
	BundleModeMaterialized BundleMode = "materialized"
	BundleModeLazy         BundleMode = "lazy"
)

type ContainerState string

type AgentInstanceState string

type SessionState string

type RunState string

const (
	ContainerRequested    ContainerState = "requested"
	ContainerProvisioning ContainerState = "provisioning"
	ContainerStarting     ContainerState = "starting"
	ContainerReady        ContainerState = "ready"
	ContainerBusy         ContainerState = "busy"
	ContainerIdle         ContainerState = "idle"
	ContainerHibernating  ContainerState = "hibernating"
	ContainerStopped      ContainerState = "stopped"
	ContainerDeleting     ContainerState = "deleting"
	ContainerDeleted      ContainerState = "deleted"
)

const (
	AgentInstalling   AgentInstanceState = "installing"
	AgentInstalled    AgentInstanceState = "installed"
	AgentStarting     AgentInstanceState = "starting"
	AgentReady        AgentInstanceState = "ready"
	AgentAuthRequired AgentInstanceState = "auth_required"
	AgentRunning      AgentInstanceState = "running"
	AgentExited       AgentInstanceState = "exited"
	AgentFailed       AgentInstanceState = "failed"
)

const (
	SessionCreating     SessionState = "creating"
	SessionActive       SessionState = "active"
	SessionWaitingInput SessionState = "waiting_input"
	SessionStreaming    SessionState = "streaming"
	SessionCancelling   SessionState = "cancelling"
	SessionCompleted    SessionState = "completed"
	SessionFailed       SessionState = "failed"
	SessionArchived     SessionState = "archived"
)

const (
	RunQueued      RunState = "queued"
	RunDispatching RunState = "dispatching"
	RunStreaming   RunState = "streaming"
	RunCompleted   RunState = "completed"
	RunCancelled   RunState = "cancelled"
	RunFailed      RunState = "failed"
)

type AgentCatalogEntry struct {
	ID           string            `json:"id"`
	Version      string            `json:"version"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Source       AgentSource       `json:"source"`
	Protocol     ACPProtocol       `json:"protocol"`
	Distribution DistributionSpec  `json:"distribution"`
	DefaultEnv   map[string]string `json:"default_env,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Platforms    []string          `json:"platforms,omitempty"`
	Private      bool              `json:"private,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type ACPProtocol struct {
	VersionRange  string     `json:"version_range,omitempty"`
	Capabilities  []string   `json:"capabilities,omitempty"`
	AuthModes     []AuthMode `json:"auth_modes,omitempty"`
	SessionList   bool       `json:"session_list,omitempty"`
	TerminalAuth  bool       `json:"terminal_auth,omitempty"`
	HostedDefault bool       `json:"hosted_default,omitempty"`
}

type DistributionSpec struct {
	Type   DistributionType     `json:"type"`
	Binary *BinaryDistribution  `json:"binary,omitempty"`
	NPX    *PackageDistribution `json:"npx,omitempty"`
	UVX    *PackageDistribution `json:"uvx,omitempty"`
}

type BinaryDistribution struct {
	URL        string   `json:"url"`
	Executable string   `json:"executable"`
	Args       []string `json:"args,omitempty"`
	SHA256     string   `json:"sha256,omitempty"`
}

type PackageDistribution struct {
	Package string   `json:"package"`
	Version string   `json:"version,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type Bundle struct {
	Digest       string            `json:"digest"`
	AgentID      string            `json:"agent_id"`
	Version      string            `json:"version"`
	Platform     string            `json:"platform"`
	Mode         BundleMode        `json:"mode"`
	Entrypoint   []string          `json:"entrypoint"`
	Env          map[string]string `json:"env,omitempty"`
	Materialized bool              `json:"materialized"`
	Manifest     DistributionSpec  `json:"manifest"`
	CreatedAt    time.Time         `json:"created_at"`
}

type ResourceLimits struct {
	CPUCount    int   `json:"cpu_count,omitempty"`
	MemoryBytes int64 `json:"memory_bytes,omitempty"`
	PIDs        int   `json:"pids,omitempty"`
}

type ContainerProfile struct {
	Image             string            `json:"image,omitempty"`
	NodeID            string            `json:"node_id,omitempty"`
	SandboxClass      string            `json:"sandbox_class,omitempty"`
	WorkspaceRoot     string            `json:"workspace_root,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	ResourceLimits    ResourceLimits    `json:"resource_limits,omitempty"`
	NetworkPolicy     string            `json:"network_policy,omitempty"`
	AllowTerminalAuth bool              `json:"allow_terminal_auth,omitempty"`
	TTL               time.Duration     `json:"ttl,omitempty"`
}

type Container struct {
	ID             string            `json:"id"`
	NodeID         string            `json:"node_id"`
	State          ContainerState    `json:"state"`
	Pinned         bool              `json:"pinned,omitempty"`
	Profile        ContainerProfile  `json:"profile"`
	BundleDigests  []string          `json:"bundle_digests,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CurrentRuns    int               `json:"current_runs"`
	Capacity       int               `json:"capacity"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	LastActiveAt   time.Time         `json:"last_active_at"`
	HibernateAfter time.Time         `json:"hibernate_after,omitempty"`
}

type ACPNegotiation struct {
	ProtocolVersion string         `json:"protocol_version,omitempty"`
	Capabilities    []string       `json:"capabilities,omitempty"`
	AuthModes       []AuthMode     `json:"auth_modes,omitempty"`
	SessionMetadata map[string]any `json:"session_metadata,omitempty"`
}

type AgentInstance struct {
	ID           string             `json:"id"`
	ContainerID  string             `json:"container_id"`
	AgentID      string             `json:"agent_id"`
	Version      string             `json:"version"`
	BundleDigest string             `json:"bundle_digest"`
	State        AgentInstanceState `json:"state"`
	Cwd          string             `json:"cwd,omitempty"`
	Env          map[string]string  `json:"env,omitempty"`
	Negotiated   ACPNegotiation     `json:"negotiated,omitempty"`
	AuthState    string             `json:"auth_state,omitempty"`
	Limits       ResourceLimits     `json:"limits,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type MCPServerSpec struct {
	Name       string            `json:"name"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	URL        string            `json:"url,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
}

type ProviderSecretRef struct {
	EnvVar string `json:"env_var,omitempty"`
	Name   string `json:"name,omitempty"`
}

type LLMProfile struct {
	ID         string            `json:"id"`
	Provider   string            `json:"provider"`
	BaseURL    string            `json:"base_url,omitempty"`
	Model      string            `json:"model"`
	DefaultEnv map[string]string `json:"default_env,omitempty"`
	APIKey     ProviderSecretRef `json:"api_key,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	ConfigPath string            `json:"config_path,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type SessionInput struct {
	Prompt      string         `json:"prompt,omitempty"`
	Attachments []string       `json:"attachments,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type SessionSpec struct {
	Metadata        map[string]any    `json:"metadata,omitempty"`
	MCPServers      []MCPServerSpec   `json:"mcp_servers,omitempty"`
	ResumeSessionID string            `json:"resume_session_id,omitempty"`
	WorkingDir      string            `json:"working_dir,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
}

type Session struct {
	ID              string          `json:"id"`
	ContainerID     string          `json:"container_id"`
	AgentInstanceID string          `json:"agent_instance_id"`
	State           SessionState    `json:"state"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
	Negotiated      ACPNegotiation  `json:"negotiated,omitempty"`
	MCPServers      []MCPServerSpec `json:"mcp_servers,omitempty"`
	ActiveRunID     string          `json:"active_run_id,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

type Run struct {
	ID              string       `json:"id"`
	SessionID       string       `json:"session_id"`
	ContainerID     string       `json:"container_id"`
	AgentInstanceID string       `json:"agent_instance_id"`
	State           RunState     `json:"state"`
	Input           SessionInput `json:"input"`
	Error           string       `json:"error,omitempty"`
	StartedAt       time.Time    `json:"started_at"`
	CompletedAt     *time.Time   `json:"completed_at,omitempty"`
	Sequence        int64        `json:"sequence,omitempty"`
}

type ExecutionPolicy struct {
	HostedMode         bool `json:"hosted_mode"`
	AllowTerminalAuth  bool `json:"allow_terminal_auth"`
	AllowPrivateAgents bool `json:"allow_private_agents"`
}

type Placement struct {
	ContainerID      string `json:"container_id,omitempty"`
	NodeID           string `json:"node_id,omitempty"`
	SchedulerChosen  bool   `json:"scheduler_chosen"`
	CreateIfNotFound bool   `json:"create_if_not_found"`
}

type ExecutionPlan struct {
	ID               string            `json:"id"`
	Agent            AgentCatalogEntry `json:"agent"`
	Bundle           Bundle            `json:"bundle"`
	ContainerProfile ContainerProfile  `json:"container_profile"`
	Session          SessionSpec       `json:"session"`
	LLMProfile       *LLMProfile       `json:"llm_profile,omitempty"`
	InjectionEnv     map[string]string `json:"injection_env,omitempty"`
	Placement        Placement         `json:"placement"`
	Policy           ExecutionPolicy   `json:"policy"`
	CreatedAt        time.Time         `json:"created_at"`
}

type AuditRecord struct {
	ID         string         `json:"id"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Action     string         `json:"action"`
	Payload    map[string]any `json:"payload,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}
