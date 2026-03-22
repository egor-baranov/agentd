package workflows

const (
	TaskQueueDefault = "agentd"

	WorkflowRegistrySync = "RegistrySyncWorkflow"
	WorkflowBundle       = "BundleWorkflow"
	WorkflowContainer    = "ContainerWorkflow"
	WorkflowSession      = "SessionWorkflow"
	WorkflowGC           = "GCWorkflow"

	ActivityRegistrySync       = "registry.sync"
	ActivityResolveBundle      = "bundle.resolve"
	ActivityProvisionContainer = "container.provision"
	ActivityEnsureAgent        = "agent.ensure"
	ActivityCreateSession      = "session.create"
	ActivityLoadSession        = "session.load"
	ActivityPrompt             = "session.prompt"
	ActivityCancel             = "session.cancel"
	ActivityHibernate          = "container.hibernate"
	ActivityDelete             = "container.delete"

	UpdateStart         = "Start"
	UpdateInstallAgent  = "InstallAgent"
	UpdateStartAgent    = "StartAgent"
	UpdateStopAgent     = "StopAgent"
	UpdateCreateSession = "CreateSession"
	UpdateLoadSession   = "LoadSession"
	UpdatePrompt        = "Prompt"
	UpdateCancel        = "Cancel"
	UpdateHibernate     = "Hibernate"
	UpdateDelete        = "Delete"

	QueryGetState     = "GetState"
	QueryListAgents   = "ListAgents"
	QueryListSessions = "ListSessions"
)
