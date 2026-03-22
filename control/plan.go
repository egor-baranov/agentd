package control

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type CompileInput struct {
	Agent            AgentCatalogEntry
	Bundle           Bundle
	ContainerProfile ContainerProfile
	Session          SessionSpec
	LLMProfile       *LLMProfile
	Placement        Placement
	Policy           ExecutionPolicy
}

func CompileExecutionPlan(input CompileInput) (ExecutionPlan, error) {
	if input.Agent.ID == "" {
		return ExecutionPlan{}, fmt.Errorf("agent id required")
	}
	if input.Bundle.Digest == "" {
		return ExecutionPlan{}, fmt.Errorf("bundle digest required")
	}
	if input.ContainerProfile.AllowTerminalAuth == false && slices.Contains(input.Agent.Protocol.AuthModes, AuthModeTerminal) && input.Policy.HostedMode {
		return ExecutionPlan{}, fmt.Errorf("%w: terminal auth disabled in hosted mode", ErrUnsupportedAuth)
	}
	plan := ExecutionPlan{
		ID:               NewID("plan"),
		Agent:            input.Agent,
		Bundle:           input.Bundle,
		ContainerProfile: input.ContainerProfile,
		Session:          input.Session,
		LLMProfile:       input.LLMProfile,
		Placement:        input.Placement,
		Policy:           input.Policy,
		CreatedAt:        time.Now().UTC(),
	}
	plan.InjectionEnv = mapsClone(input.Agent.DefaultEnv)
	for k, v := range input.Session.Env {
		plan.InjectionEnv[k] = v
	}
	for k, v := range injectLLMProfileEnv(input.LLMProfile) {
		plan.InjectionEnv[k] = v
	}
	if plan.ContainerProfile.WorkspaceRoot == "" {
		plan.ContainerProfile.WorkspaceRoot = "/workspace"
	}
	if plan.ContainerProfile.NetworkPolicy == "" {
		plan.ContainerProfile.NetworkPolicy = "default"
	}
	if plan.ContainerProfile.TTL == 0 {
		plan.ContainerProfile.TTL = 15 * time.Minute
	}
	if len(plan.Bundle.Entrypoint) == 0 {
		return ExecutionPlan{}, fmt.Errorf("bundle %s has no entrypoint", plan.Bundle.Digest)
	}
	if input.Policy.HostedMode {
		plan.ContainerProfile.AllowTerminalAuth = input.Policy.AllowTerminalAuth && plan.ContainerProfile.AllowTerminalAuth
	}
	if strings.TrimSpace(plan.Session.WorkingDir) == "" {
		plan.Session.WorkingDir = plan.ContainerProfile.WorkspaceRoot
	}
	return plan, nil
}

func mapsClone(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func injectLLMProfileEnv(profile *LLMProfile) map[string]string {
	if profile == nil {
		return map[string]string{}
	}
	env := map[string]string{
		"ACP_LLM_PROVIDER": profile.Provider,
		"ACP_LLM_MODEL":    profile.Model,
	}
	if profile.BaseURL != "" {
		env["ACP_LLM_BASE_URL"] = profile.BaseURL
	}
	for k, v := range profile.DefaultEnv {
		env[k] = v
	}
	if profile.APIKey.EnvVar != "" {
		env["ACP_LLM_API_KEY_ENV"] = profile.APIKey.EnvVar
	}
	if profile.APIKey.Name != "" {
		env["ACP_LLM_API_KEY_REF"] = profile.APIKey.Name
	}
	return env
}
