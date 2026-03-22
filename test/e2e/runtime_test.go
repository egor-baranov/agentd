package e2e_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	runtimepkg "runtime"
	"testing"
	"time"

	"agentd/bundle"
	"agentd/control"
	localruntime "agentd/runtime/local"
	"agentd/session"
)

func TestRunnerLifecycleWithEchoAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	binary := buildEchoAgent(t)
	runner := localruntime.NewRunner("node-test", t.TempDir(), slog.Default())
	container, err := runner.ProvisionContainer(ctx, control.CreateContainerRequest{})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	resolver := bundle.Resolver{}
	agentA := manualBinaryAgent("echo-a", binary)
	bundleA, _ := resolver.Resolve(agentA, "")
	planA, err := control.CompileExecutionPlan(control.CompileInput{Agent: agentA, Bundle: bundleA, ContainerProfile: container.Profile, Session: control.SessionSpec{WorkingDir: container.Profile.WorkspaceRoot}, Policy: control.ExecutionPolicy{HostedMode: false, AllowPrivateAgents: true}})
	if err != nil {
		t.Fatalf("compile plan a: %v", err)
	}
	instA, err := runner.EnsureAgent(ctx, control.InstallAgentRequest{ContainerID: container.ID, Plan: planA})
	if err != nil {
		t.Fatalf("ensure agent a: %v", err)
	}
	agentB := manualBinaryAgent("echo-b", binary)
	bundleB, _ := resolver.Resolve(agentB, "")
	planB, err := control.CompileExecutionPlan(control.CompileInput{Agent: agentB, Bundle: bundleB, ContainerProfile: container.Profile, Session: control.SessionSpec{WorkingDir: container.Profile.WorkspaceRoot}, Policy: control.ExecutionPolicy{HostedMode: false, AllowPrivateAgents: true}})
	if err != nil {
		t.Fatalf("compile plan b: %v", err)
	}
	instB, err := runner.EnsureAgent(ctx, control.InstallAgentRequest{ContainerID: container.ID, Plan: planB})
	if err != nil {
		t.Fatalf("ensure agent b: %v", err)
	}
	snap, err := runner.GetContainer(ctx, container.ID)
	if err != nil {
		t.Fatalf("get container: %v", err)
	}
	if len(snap.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(snap.Agents))
	}
	sessA, err := runner.CreateSession(ctx, control.CreateSessionRequest{ContainerID: container.ID, AgentInstanceID: instA.ID, Plan: planA})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	var events []session.Event
	run, err := runner.Prompt(ctx, control.PromptRequest{SessionID: sessA.ID, RunID: "run-complete", Input: control.SessionInput{Prompt: "hello"}}, func(event session.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if run.State != control.RunCompleted || len(events) == 0 {
		t.Fatalf("unexpected run/events: %+v len=%d", run, len(events))
	}
	sessB, err := runner.CreateSession(ctx, control.CreateSessionRequest{ContainerID: container.ID, AgentInstanceID: instB.ID, Plan: planB})
	if err != nil {
		t.Fatalf("create session b: %v", err)
	}
	resultCh := make(chan control.Run, 1)
	errCh := make(chan error, 1)
	go func() {
		r, err := runner.Prompt(ctx, control.PromptRequest{SessionID: sessB.ID, RunID: "run-cancel", Input: control.SessionInput{Prompt: "cancel me"}}, nil)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- r
	}()
	time.Sleep(20 * time.Millisecond)
	if err := runner.Cancel(ctx, control.CancelRequest{SessionID: sessB.ID, RunID: "run-cancel"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("prompt error: %v", err)
	case run := <-resultCh:
		if run.State != control.RunCancelled {
			t.Fatalf("expected cancelled run, got %+v", run)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for cancelled run")
	}
}

func buildEchoAgent(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "../.."))
	bin := filepath.Join(t.TempDir(), "echo-agent")
	goBin := filepath.Join(runtimepkg.GOROOT(), "bin", "go")
	cmd := exec.Command(goBin, "build", "-o", bin, "./examples/echo-agent")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build echo agent: %v\n%s", err, out)
	}
	return bin
}

func manualBinaryAgent(id, executable string) control.AgentCatalogEntry {
	return control.AgentCatalogEntry{
		ID:           id,
		Version:      "1.0.0",
		Source:       control.AgentSourceManual,
		Protocol:     control.ACPProtocol{AuthModes: []control.AuthMode{control.AuthModeAgent}},
		Distribution: control.DistributionSpec{Type: control.DistributionBinary, Binary: &control.BinaryDistribution{URL: "https://example.invalid/agent", Executable: executable}},
	}
}
