package workflows_test

import (
	"context"
	"testing"
	"time"

	"agentd/control"
	"agentd/workflows"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestContainerWorkflowUpdates(t *testing.T) {
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(workflows.ContainerWorkflow)
	env.RegisterWorkflow(workflows.SessionWorkflow)
	var started control.Container
	var installed control.AgentInstance
	var created control.Session
	env.RegisterActivityWithOptions(func(context.Context, control.CreateContainerRequest) (control.Container, error) {
		return control.Container{ID: "ctr-1", State: control.ContainerReady}, nil
	}, activity.RegisterOptions{Name: workflows.ActivityProvisionContainer})
	env.RegisterActivityWithOptions(func(context.Context, control.InstallAgentRequest) (control.AgentInstance, error) {
		return control.AgentInstance{ID: "agent-1", ContainerID: "ctr-1", State: control.AgentReady}, nil
	}, activity.RegisterOptions{Name: workflows.ActivityEnsureAgent})
	env.RegisterActivityWithOptions(func(context.Context, control.CreateSessionRequest) (control.Session, error) {
		return control.Session{ID: "sess-1", ContainerID: "ctr-1", AgentInstanceID: "agent-1", State: control.SessionActive}, nil
	}, activity.RegisterOptions{Name: workflows.ActivityCreateSession})
	env.RegisterActivityWithOptions(func(context.Context, string) error { return nil }, activity.RegisterOptions{Name: workflows.ActivityDelete})
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(workflows.UpdateStart, "u1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { t.Fatalf("start rejected: %v", err) },
			OnComplete: func(v interface{}, err error) {
				if err != nil {
					t.Fatalf("start complete: %v", err)
				}
				started = v.(control.Container)
			},
		}, control.CreateContainerRequest{ID: "ctr-1"})
	}, time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(workflows.UpdateInstallAgent, "u2", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { t.Fatalf("install rejected: %v", err) },
			OnComplete: func(v interface{}, err error) {
				if err != nil {
					t.Fatalf("install complete: %v", err)
				}
				installed = v.(control.AgentInstance)
			},
		}, control.InstallAgentRequest{ContainerID: "ctr-1"})
	}, 2*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(workflows.UpdateCreateSession, "u3", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { t.Fatalf("session rejected: %v", err) },
			OnComplete: func(v interface{}, err error) {
				if err != nil {
					t.Fatalf("session complete: %v", err)
				}
				created = v.(control.Session)
			},
		}, control.CreateSessionRequest{ContainerID: "ctr-1", AgentInstanceID: "agent-1"})
	}, 3*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(workflows.UpdateDelete, "u4", &testsuite.TestUpdateCallback{OnReject: func(err error) { t.Fatalf("delete rejected: %v", err) }})
	}, 4*time.Millisecond)
	env.ExecuteWorkflow(workflows.ContainerWorkflow, workflows.ContainerWorkflowInput{ContainerID: "ctr-1"})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow should complete after delete")
	}
	if started.ID != "ctr-1" || installed.ID != "agent-1" || created.ID != "sess-1" {
		t.Fatalf("unexpected update results: %+v %+v %+v", started, installed, created)
	}
}

func TestSessionWorkflowPromptUpdate(t *testing.T) {
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(workflows.SessionWorkflow)
	var run control.Run
	env.RegisterActivityWithOptions(func(context.Context, control.PromptRequest) (control.Run, error) {
		return control.Run{ID: "run-1", SessionID: "sess-1", State: control.RunCompleted}, nil
	}, activity.RegisterOptions{Name: workflows.ActivityPrompt})
	env.RegisterActivityWithOptions(func(context.Context, control.CancelRequest) error { return nil }, activity.RegisterOptions{Name: workflows.ActivityCancel})
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(workflows.UpdatePrompt, "u1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { t.Fatalf("prompt rejected: %v", err) },
			OnComplete: func(v interface{}, err error) {
				if err != nil {
					t.Fatalf("prompt complete: %v", err)
				}
				run = v.(control.Run)
			},
		}, control.PromptRequest{SessionID: "sess-1"})
	}, time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.CancelWorkflow()
	}, 2*time.Millisecond)
	env.ExecuteWorkflow(workflows.SessionWorkflow, workflows.SessionWorkflowInput{Session: control.Session{ID: "sess-1", State: control.SessionActive}})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow should complete after cancel")
	}
	if run.ID != "run-1" || run.State != control.RunCompleted {
		t.Fatalf("unexpected run result: %+v", run)
	}
}
