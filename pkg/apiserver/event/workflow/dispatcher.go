package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/domain/repository"
	msg "kubemin-cli/pkg/apiserver/infrastructure/messaging"
)

// Note: Worker resilience constants are defined in config/consts.go:
// - config.DefaultWorkerBackoffMin
// - config.DefaultWorkerBackoffMax
// - config.DefaultWorkerMaxReadFailures
// - config.DefaultWorkerMaxClaimFailures

// TaskDispatch is the minimal payload for dispatching a workflow task to a worker.
type TaskDispatch struct {
	TaskID     string `json:"task_id"`
	WorkflowID string `json:"workflow_id"`
	ProjectID  string `json:"project_id"`
	AppID      string `json:"app_id"`
}

func MarshalTaskDispatch(t TaskDispatch) ([]byte, error) { return json.Marshal(t) }
func UnmarshalTaskDispatch(b []byte) (TaskDispatch, error) {
	var t TaskDispatch
	err := json.Unmarshal(b, &t)
	return t, err
}

// WorkflowTaskSender is the original local executor scanning DB and running tasks.
func (w *Workflow) WorkflowTaskSender(ctx context.Context) {
	ticker := time.NewTicker(w.localPollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.V(3).Info("workflow task sender stopped: context cancelled")
			return
		case <-ticker.C:
		}

		waitingTasks, err := w.waitingTasks(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			klog.Errorf("list waiting workflow tasks failed: %v", err)
			continue
		}
		if len(waitingTasks) == 0 {
			continue
		}
		for _, task := range waitingTasks {
			if ctx.Err() != nil {
				return
			}
			w.claimAndProcessTask(ctx, task, func(procCtx context.Context, queuedTask *model.WorkflowQueue) error {
				return w.updateQueueAndRunTask(procCtx, queuedTask, 1)
			})
		}
	}
}

// Dispatcher scans waiting tasks and publishes dispatch messages.
func (w *Workflow) Dispatcher(ctx context.Context) {
	ticker := time.NewTicker(w.dispatchPollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.V(3).Info("workflow dispatcher stopped: context cancelled")
			return
		case <-ticker.C:
		}

		waitingTasks, err := w.waitingTasks(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			klog.Errorf("list waiting workflow tasks failed: %v", err)
			continue
		}
		if len(waitingTasks) == 0 {
			continue
		}
		for _, task := range waitingTasks {
			if ctx.Err() != nil {
				return
			}
			w.claimAndProcessTask(ctx, task, func(procCtx context.Context, queuedTask *model.WorkflowQueue) error {
				payload := TaskDispatch{
					TaskID:     queuedTask.TaskID,
					WorkflowID: queuedTask.WorkflowID,
					ProjectID:  queuedTask.ProjectID,
					AppID:      queuedTask.AppID,
				}
				b, err := MarshalTaskDispatch(payload)
				if err != nil {
					return fmt.Errorf("marshal task dispatch: %w", err)
				}
				id, err := w.enqueueDispatch(procCtx, b)
				if err != nil {
					return fmt.Errorf("enqueue task dispatch: %w", err)
				}
				klog.Infof("dispatched task: %s, streamID: %s", queuedTask.TaskID, id)
				return nil
			})
		}
	}
}

// StartWorker subscribes to task dispatch topic and executes tasks.
// It implements resilient behavior: by default (max failures = 0), it retries indefinitely
// with exponential backoff instead of exiting on transient errors.
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
	w.errChan = errChan
	group := w.consumerGroup()
	consumer := w.consumerName()

	// Get config values with defaults
	backoffMin := w.workerBackoffMin()
	backoffMax := w.workerBackoffMax()
	maxReadFailures := w.workerMaxReadFailures()
	maxClaimFailures := w.workerMaxClaimFailures()

	klog.Infof("worker starting: stream=%s group=%s consumer=%s maxReadFailures=%d maxClaimFailures=%d",
		w.dispatchTopic(), group, consumer, maxReadFailures, maxClaimFailures)

	// Ensure consumer group exists once on worker start to avoid per-read overhead.
	if err := w.Queue.EnsureGroup(ctx, group); err != nil {
		klog.V(4).Infof("ensure group error: %v", err)
	}

	staleTicker := time.NewTicker(w.workerStaleInterval())
	defer staleTicker.Stop()

	currentDelay := backoffMin
	readFailures := 0
	claimFailures := 0

	for {
		select {
		case <-ctx.Done():
			klog.Info("worker shutting down due to context cancellation")
			return
		case <-staleTicker.C:
			mags, err := w.Queue.AutoClaim(ctx, group, consumer, w.workerAutoClaimMinIdle(), w.workerAutoClaimCount())
			if err != nil {
				claimFailures++
				klog.Warningf("auto-claim error (consecutive: %d): %v", claimFailures, err)
				w.reportWorkerError(fmt.Errorf("auto-claim failed (%d consecutive): %w", claimFailures, err))

				// Only exit if maxClaimFailures > 0 and threshold reached
				if maxClaimFailures > 0 && claimFailures >= maxClaimFailures {
					klog.Errorf("max claim failures reached (%d), worker exiting", maxClaimFailures)
					return
				}
				continue
			}
			claimFailures = 0
			currentDelay = backoffMin
			var acknowledgements []dispatchAck
			for _, m := range mags {
				if ack, taskID := w.processDispatchMessage(ctx, m); ack {
					acknowledgements = append(acknowledgements, dispatchAck{id: m.ID, taskID: taskID})
				} else {
					klog.Warningf("consumer=%s left message pending id=%s task=%s due to processing error", consumer, m.ID, taskID)
				}
			}
			w.ackDispatchMessages(ctx, group, consumer, acknowledgements)
		default:
			mags, err := w.Queue.ReadGroup(ctx, group, consumer, w.workerReadCount(), w.workerReadBlock())
			if err != nil {
				readFailures++
				klog.Warningf("read group error (consecutive: %d): %v", readFailures, err)
				w.reportWorkerError(fmt.Errorf("read group failed (%d consecutive): %w", readFailures, err))

				// Exponential backoff
				wait := w.workerBackoffDelay(currentDelay, backoffMin, backoffMax)
				currentDelay = wait

				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}

				// Only exit if maxReadFailures > 0 and threshold reached
				if maxReadFailures > 0 && readFailures >= maxReadFailures {
					klog.Errorf("max read failures reached (%d), worker exiting", maxReadFailures)
					return
				}
				continue
			}
			readFailures = 0
			currentDelay = backoffMin
			var acknowledgements []dispatchAck
			for _, m := range mags {
				if ack, taskID := w.processDispatchMessage(ctx, m); ack {
					acknowledgements = append(acknowledgements, dispatchAck{id: m.ID, taskID: taskID, claimed: true})
				} else {
					klog.Warningf("consumer=%s left claimed message pending id=%s task=%s due to processing error", consumer, m.ID, taskID)
				}
			}
			w.ackDispatchMessages(ctx, group, consumer, acknowledgements)
		}
	}
}

func (w *Workflow) claimAndProcessTask(ctx context.Context, task *model.WorkflowQueue, processor func(context.Context, *model.WorkflowQueue) error) {
	claimed, err := w.markTaskStatus(ctx, task.TaskID, config.StatusWaiting, config.StatusQueued)
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}
		klog.Errorf("mark task %s queued failed: %v", task.TaskID, err)
		return
	}
	if !claimed {
		klog.V(4).Infof("task %s already claimed before mark queued", task.TaskID)
		return
	}
	if err := processor(ctx, task); err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}
		klog.Errorf("process task %s failed: %v", task.TaskID, err)
		if reverted, revertErr := w.markTaskStatus(ctx, task.TaskID, config.StatusQueued, config.StatusWaiting); revertErr != nil {
			if errors.Is(revertErr, context.Canceled) || ctx.Err() != nil {
				return
			}
			klog.Errorf("revert task %s status to waiting failed: %v", task.TaskID, revertErr)
		} else if !reverted {
			klog.V(4).Infof("task %s status already changed before revert", task.TaskID)
		}
	}
}

func (w *Workflow) ackMessage(ctx context.Context, group string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	return w.Queue.Ack(ctx, group, ids...)
}

type dispatchAck struct {
	id      string
	taskID  string
	claimed bool
}

func (w *Workflow) ackDispatchMessages(ctx context.Context, group, consumer string, acks []dispatchAck) {
	if len(acks) == 0 {
		return
	}
	ids := make([]string, 0, len(acks))
	for _, ack := range acks {
		ids = append(ids, ack.id)
	}
	if err := w.ackMessage(ctx, group, ids...); err != nil {
		for _, ack := range acks {
			if ack.claimed {
				klog.Errorf("failed to ack claimed message id=%s task=%s: %v", ack.id, ack.taskID, err)
			} else {
				klog.Errorf("failed to ack message id=%s task=%s: %v", ack.id, ack.taskID, err)
			}
		}
		return
	}
	for _, ack := range acks {
		if ack.claimed {
			klog.Infof("consumer=%s acked claimed message id=%s task=%s", consumer, ack.id, ack.taskID)
		} else {
			klog.Infof("consumer=%s acked message id=%s task=%s", consumer, ack.id, ack.taskID)
		}
	}
}

func (w *Workflow) workerBackoffDelay(current, min, max time.Duration) time.Duration {
	if current < min {
		return min
	}
	next := current * 2
	if next > max {
		return max
	}
	return next
}

func (w *Workflow) reportWorkerError(err error) {
	if err == nil {
		return
	}
	w.reportTaskError(err)
}

// processDispatchMessage processes a single dispatch message.
// This is a "pass/fail" system - no retries. Failures are logged and the message is acknowledged.
// Task state is tracked in the database, so operators can see failed tasks there.
func (w *Workflow) processDispatchMessage(ctx context.Context, m msg.Message) (bool, string) {
	td, err := UnmarshalTaskDispatch(m.Payload)
	if err != nil {
		// Parse error: log and ack to prevent blocking
		klog.Errorf("decode dispatch failed: %v, payload: %s", err, string(m.Payload))
		return true, ""
	}

	task, err := repository.TaskByID(ctx, w.Store, td.TaskID)
	if err != nil {
		// DB error: log and ack
		klog.Errorf("load task %s failed: %v", td.TaskID, err)
		return true, td.TaskID
	}

	if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
		// Execution error: task status is already updated in updateQueueAndRunTask
		klog.Errorf("run task %s failed: %v", td.TaskID, err)
	}

	return true, td.TaskID
}

func (w *Workflow) dispatchTopic() string {
	prefix := ""
	if w.Cfg != nil {
		prefix = w.Cfg.Messaging.ChannelPrefix
	}
	if prefix == "" {
		prefix = "kubemin"
	}
	return fmt.Sprintf("%s.workflow.dispatch", prefix)
}

func (w *Workflow) consumerGroup() string { return "workflow-workers" }
func (w *Workflow) consumerName() string {
	if w.Cfg != nil {
		return w.Cfg.LeaderConfig.ID
	}
	return "worker"
}
