package workflows

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func Register(w worker.Worker, activities *Activities) {
	w.RegisterWorkflowWithOptions(RegistrySyncWorkflow, workflow.RegisterOptions{Name: WorkflowRegistrySync})
	w.RegisterWorkflowWithOptions(BundleWorkflow, workflow.RegisterOptions{Name: WorkflowBundle})
	w.RegisterWorkflowWithOptions(ContainerWorkflow, workflow.RegisterOptions{Name: WorkflowContainer})
	w.RegisterWorkflowWithOptions(SessionWorkflow, workflow.RegisterOptions{Name: WorkflowSession})
	w.RegisterWorkflowWithOptions(GCWorkflow, workflow.RegisterOptions{Name: WorkflowGC})
	w.RegisterActivityWithOptions(activities.RegistrySync, activity.RegisterOptions{Name: ActivityRegistrySync})
	w.RegisterActivityWithOptions(activities.ResolveBundle, activity.RegisterOptions{Name: ActivityResolveBundle})
	w.RegisterActivityWithOptions(activities.ProvisionContainer, activity.RegisterOptions{Name: ActivityProvisionContainer})
	w.RegisterActivityWithOptions(activities.EnsureAgent, activity.RegisterOptions{Name: ActivityEnsureAgent})
	w.RegisterActivityWithOptions(activities.CreateSession, activity.RegisterOptions{Name: ActivityCreateSession})
	w.RegisterActivityWithOptions(activities.LoadSession, activity.RegisterOptions{Name: ActivityLoadSession})
	w.RegisterActivityWithOptions(activities.Prompt, activity.RegisterOptions{Name: ActivityPrompt})
	w.RegisterActivityWithOptions(activities.Cancel, activity.RegisterOptions{Name: ActivityCancel})
	w.RegisterActivityWithOptions(activities.Hibernate, activity.RegisterOptions{Name: ActivityHibernate})
	w.RegisterActivityWithOptions(activities.Delete, activity.RegisterOptions{Name: ActivityDelete})
}
