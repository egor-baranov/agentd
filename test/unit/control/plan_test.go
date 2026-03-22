package control_test

import (
	"testing"

	"agentd/control"
)

func TestCompileExecutionPlanInjectsDefaults(t *testing.T) {
	plan, err := control.CompileExecutionPlan(control.CompileInput{
		Agent:      control.AgentCatalogEntry{ID: "agent", Version: "1.0.0", DefaultEnv: map[string]string{"A": "1"}},
		Bundle:     control.Bundle{Digest: "sha256:abc", Entrypoint: []string{"agent"}},
		Session:    control.SessionSpec{Env: map[string]string{"B": "2"}},
		LLMProfile: &control.LLMProfile{Provider: "openai", Model: "gpt-test", APIKey: control.ProviderSecretRef{EnvVar: "OPENAI_API_KEY"}},
		Policy:     control.ExecutionPolicy{HostedMode: true, AllowPrivateAgents: true},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if plan.ContainerProfile.WorkspaceRoot != "/workspace" {
		t.Fatalf("unexpected workspace root: %q", plan.ContainerProfile.WorkspaceRoot)
	}
	if plan.InjectionEnv["A"] != "1" || plan.InjectionEnv["B"] != "2" || plan.InjectionEnv["ACP_LLM_PROVIDER"] != "openai" {
		t.Fatalf("missing env injection: %+v", plan.InjectionEnv)
	}
}
