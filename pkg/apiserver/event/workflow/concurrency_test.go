package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
)

func TestDetermineStepConcurrency(t *testing.T) {
	testCases := []struct {
		name     string
		mode     config.WorkflowMode
		jobCount int
		limit    int
		want     int
	}{
		{
			name:     "sequential_respects_limit",
			mode:     config.WorkflowModeStepByStep,
			jobCount: 5,
			limit:    2,
			want:     2,
		},
		{
			name:     "sequential_uses_job_count_when_smaller",
			mode:     config.WorkflowModeStepByStep,
			jobCount: 3,
			limit:    8,
			want:     3,
		},
		{
			name:     "sequential_falls_back_to_one",
			mode:     config.WorkflowModeStepByStep,
			jobCount: 4,
			limit:    0,
			want:     1,
		},
		{
			name:     "parallel_ignores_limit",
			mode:     config.WorkflowModeDAG,
			jobCount: 6,
			limit:    2,
			want:     6,
		},
		{
			name:     "zero_jobs_returns_zero",
			mode:     config.WorkflowModeStepByStep,
			jobCount: 0,
			limit:    4,
			want:     0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := determineStepConcurrency(tc.mode, tc.jobCount, tc.limit)
			require.Equal(t, tc.want, got)
		})
	}
}
