package control

import "fmt"

var containerTransitions = map[ContainerState]map[ContainerState]struct{}{
	ContainerRequested:    {ContainerProvisioning: {}, ContainerDeleting: {}},
	ContainerProvisioning: {ContainerStarting: {}, ContainerDeleting: {}},
	ContainerStarting:     {ContainerReady: {}, ContainerStopped: {}, ContainerDeleting: {}},
	ContainerReady:        {ContainerBusy: {}, ContainerIdle: {}, ContainerHibernating: {}, ContainerDeleting: {}},
	ContainerBusy:         {ContainerReady: {}, ContainerIdle: {}, ContainerHibernating: {}, ContainerDeleting: {}},
	ContainerIdle:         {ContainerReady: {}, ContainerBusy: {}, ContainerHibernating: {}, ContainerDeleting: {}},
	ContainerHibernating:  {ContainerStopped: {}, ContainerReady: {}, ContainerDeleting: {}},
	ContainerStopped:      {ContainerStarting: {}, ContainerDeleting: {}},
	ContainerDeleting:     {ContainerDeleted: {}},
	ContainerDeleted:      {},
}

var agentTransitions = map[AgentInstanceState]map[AgentInstanceState]struct{}{
	AgentInstalling:   {AgentInstalled: {}, AgentFailed: {}},
	AgentInstalled:    {AgentStarting: {}, AgentFailed: {}},
	AgentStarting:     {AgentReady: {}, AgentAuthRequired: {}, AgentFailed: {}},
	AgentReady:        {AgentRunning: {}, AgentAuthRequired: {}, AgentExited: {}, AgentFailed: {}},
	AgentAuthRequired: {AgentStarting: {}, AgentReady: {}, AgentFailed: {}},
	AgentRunning:      {AgentReady: {}, AgentExited: {}, AgentFailed: {}},
	AgentExited:       {AgentStarting: {}, AgentFailed: {}},
	AgentFailed:       {AgentInstalling: {}, AgentStarting: {}},
}

var sessionTransitions = map[SessionState]map[SessionState]struct{}{
	SessionCreating:     {SessionActive: {}, SessionFailed: {}},
	SessionActive:       {SessionWaitingInput: {}, SessionStreaming: {}, SessionCancelling: {}, SessionCompleted: {}, SessionFailed: {}, SessionArchived: {}},
	SessionWaitingInput: {SessionStreaming: {}, SessionCancelling: {}, SessionCompleted: {}, SessionFailed: {}, SessionArchived: {}},
	SessionStreaming:    {SessionWaitingInput: {}, SessionActive: {}, SessionCancelling: {}, SessionCompleted: {}, SessionFailed: {}},
	SessionCancelling:   {SessionCompleted: {}, SessionFailed: {}, SessionArchived: {}},
	SessionCompleted:    {SessionArchived: {}, SessionActive: {}},
	SessionFailed:       {SessionArchived: {}, SessionActive: {}},
	SessionArchived:     {},
}

var runTransitions = map[RunState]map[RunState]struct{}{
	RunQueued:      {RunDispatching: {}, RunCancelled: {}, RunFailed: {}},
	RunDispatching: {RunStreaming: {}, RunCompleted: {}, RunCancelled: {}, RunFailed: {}},
	RunStreaming:   {RunCompleted: {}, RunCancelled: {}, RunFailed: {}},
	RunCompleted:   {},
	RunCancelled:   {},
	RunFailed:      {},
}

func ValidateContainerTransition(current, next ContainerState) error {
	return validateTransition("container", current, next, containerTransitions[current])
}

func ValidateAgentTransition(current, next AgentInstanceState) error {
	return validateTransition("agent", current, next, agentTransitions[current])
}

func ValidateSessionTransition(current, next SessionState) error {
	return validateTransition("session", current, next, sessionTransitions[current])
}

func ValidateRunTransition(current, next RunState) error {
	return validateTransition("run", current, next, runTransitions[current])
}

func validateTransition[T ~string](kind string, current, next T, allowed map[T]struct{}) error {
	if current == next {
		return nil
	}
	if _, ok := allowed[next]; ok {
		return nil
	}
	return fmt.Errorf("invalid %s transition: %s -> %s", kind, string(current), string(next))
}
