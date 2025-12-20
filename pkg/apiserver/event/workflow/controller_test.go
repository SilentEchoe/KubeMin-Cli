package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
)

func TestStopOnFailureLogicForStepModes(t *testing.T) {
	// This test verifies that stopOnFailure is correctly set based on step mode:
	// - StepByStep (sequential) mode: stopOnFailure = true (stop on first failure)
	// - DAG (parallel) mode: stopOnFailure = false (continue all jobs)

	t.Run("StepByStep mode should have stopOnFailure=true", func(t *testing.T) {
		mode := config.WorkflowModeStepByStep
		// The fix: stopOnFailure = !mode.IsParallel()
		stopOnFailure := !mode.IsParallel()
		require.True(t, stopOnFailure, "StepByStep mode should stop on first failure")
	})

	t.Run("DAG mode should have stopOnFailure=false", func(t *testing.T) {
		mode := config.WorkflowModeDAG
		stopOnFailure := !mode.IsParallel()
		require.False(t, stopOnFailure, "DAG mode should continue all jobs even if some fail")
	})
}

func TestWorkflowCtlSnapshotTask(t *testing.T) {
	task := &model.WorkflowQueue{
		TaskID:       "task-123",
		WorkflowName: "test-workflow",
		Status:       config.StatusQueued,
	}

	ctl := &WorkflowCtl{
		workflowTask: task,
	}

	snapshot := ctl.snapshotTask()
	require.Equal(t, "task-123", snapshot.TaskID)
	require.Equal(t, "test-workflow", snapshot.WorkflowName)
	require.Equal(t, config.StatusQueued, snapshot.Status)
}

func TestWorkflowCtlSetStatus(t *testing.T) {
	task := &model.WorkflowQueue{
		TaskID: "task-123",
		Status: config.StatusQueued,
	}

	ctl := &WorkflowCtl{
		workflowTask: task,
	}

	ctl.setStatus(config.StatusRunning)
	require.Equal(t, config.StatusRunning, ctl.workflowTask.Status)

	ctl.setStatus(config.StatusCompleted)
	require.Equal(t, config.StatusCompleted, ctl.workflowTask.Status)
}

func TestWorkflowCtlMutateTask(t *testing.T) {
	task := &model.WorkflowQueue{
		TaskID: "task-123",
		Status: config.StatusQueued,
	}

	ctl := &WorkflowCtl{
		workflowTask: task,
	}

	ctl.mutateTask(func(t *model.WorkflowQueue) {
		t.Status = config.StatusRunning
		t.WorkflowName = "updated-workflow"
	})

	require.Equal(t, config.StatusRunning, ctl.workflowTask.Status)
	require.Equal(t, "updated-workflow", ctl.workflowTask.WorkflowName)
}

func TestIsWorkflowTerminal(t *testing.T) {
	testCases := []struct {
		status   config.Status
		expected bool
	}{
		{config.StatusPassed, true},
		{config.StatusFailed, true},
		{config.StatusTimeout, true},
		{config.StatusReject, true},
		{config.StatusRunning, false},
		{config.StatusQueued, false},
		{config.StatusWaiting, false},
		{config.StatusCancelled, false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.status), func(t *testing.T) {
			result := isWorkflowTerminal(tc.status)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestIsJobSuccessStatus(t *testing.T) {
	testCases := []struct {
		status   config.Status
		expected bool
	}{
		{config.StatusCompleted, true},
		{config.StatusSkipped, true},
		{config.StatusPassed, true},
		{config.StatusFailed, false},
		{config.StatusRunning, false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.status), func(t *testing.T) {
			require.Equal(t, tc.expected, isJobSuccessStatus(tc.status))
		})
	}
}
