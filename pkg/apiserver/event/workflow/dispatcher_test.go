package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
	msg "KubeMin-Cli/pkg/apiserver/infrastructure/messaging"
)

type fakeAckQueue struct {
	ackErr   error
	ackCalls []ackRequest
}

type ackRequest struct {
	group string
	ids   []string
}

func (f *fakeAckQueue) record(group string, ids ...string) {
	copied := append([]string(nil), ids...)
	f.ackCalls = append(f.ackCalls, ackRequest{group: group, ids: copied})
}

func (f *fakeAckQueue) EnsureGroup(context.Context, string) error       { return nil }
func (f *fakeAckQueue) Enqueue(context.Context, []byte) (string, error) { return "", nil }
func (f *fakeAckQueue) ReadGroup(context.Context, string, string, int, time.Duration) ([]msg.Message, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAckQueue) Ack(ctx context.Context, group string, ids ...string) error {
	f.record(group, ids...)
	return f.ackErr
}
func (f *fakeAckQueue) AutoClaim(context.Context, string, string, time.Duration, int) ([]msg.Message, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAckQueue) Close(context.Context) error                         { return nil }
func (f *fakeAckQueue) Stats(context.Context, string) (int64, int64, error) { return 0, 0, nil }

func TestAckMessageDelegatesToQueue(t *testing.T) {
	q := &fakeAckQueue{}
	wf := &Workflow{
		Queue: q,
		Cfg: &config.Config{
			Workflow: config.WorkflowRuntimeConfig{MaxConcurrentWorkflows: 1},
		},
	}

	err := wf.ackMessage(context.Background(), "workflow-workers", "1", "2", "3")
	require.NoError(t, err)
	require.Len(t, q.ackCalls, 1)
	require.Equal(t, "workflow-workers", q.ackCalls[0].group)
	require.Equal(t, []string{"1", "2", "3"}, q.ackCalls[0].ids)
}

func TestAckMessagePropagatesError(t *testing.T) {
	expectedErr := errors.New("ack failed")
	q := &fakeAckQueue{ackErr: expectedErr}
	wf := &Workflow{Queue: q}

	err := wf.ackMessage(context.Background(), "workflow-workers", "42")
	require.ErrorIs(t, err, expectedErr)
	require.Len(t, q.ackCalls, 1)
}

func TestAckDispatchMessagesBatchesIDs(t *testing.T) {
	q := &fakeAckQueue{}
	wf := &Workflow{Queue: q}
	acks := []dispatchAck{
		{id: "1", taskID: "task-1"},
		{id: "2", taskID: "task-2", claimed: true},
	}

	wf.ackDispatchMessages(context.Background(), "workflow-workers", "consumer-1", acks)

	require.Len(t, q.ackCalls, 1)
	require.Equal(t, "workflow-workers", q.ackCalls[0].group)
	require.Equal(t, []string{"1", "2"}, q.ackCalls[0].ids)
}

func TestAckDispatchMessagesSkipsEmptyBatch(t *testing.T) {
	q := &fakeAckQueue{}
	wf := &Workflow{Queue: q}

	wf.ackDispatchMessages(context.Background(), "workflow-workers", "consumer-1", nil)

	require.Len(t, q.ackCalls, 0)
}

func TestWorkerBackoffDelay(t *testing.T) {
	wf := &Workflow{}
	testCases := []struct {
		name     string
		current  time.Duration
		expected time.Duration
	}{
		{"lessThanMin", 0, 200 * time.Millisecond},
		{"doubleWithinMax", 400 * time.Millisecond, 800 * time.Millisecond},
		{"capAtMax", 4 * time.Second, 5 * time.Second},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := wf.workerBackoffDelay(tc.current, 200*time.Millisecond, 5*time.Second)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestReportWorkerErrorSendsToErrChan(t *testing.T) {
	wf := &Workflow{
		errChan: make(chan error, 1),
	}
	wf.reportWorkerError(errors.New("worker failed"))
	select {
	case err := <-wf.errChan:
		require.Error(t, err)
		require.Contains(t, err.Error(), "worker failed")
	default:
		t.Fatalf("expected error to be sent to errChan")
	}
}

func TestReportWorkerErrorIgnoresNil(t *testing.T) {
	wf := &Workflow{
		errChan: make(chan error, 1),
	}
	wf.reportWorkerError(nil)
	require.Len(t, wf.errChan, 0)
}
