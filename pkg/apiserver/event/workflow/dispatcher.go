package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	msg "KubeMin-Cli/pkg/apiserver/infrastructure/messaging"
)

// TaskDispatch is the minimal payload for dispatching a workflow task to a worker.
type TaskDispatch struct {
	TaskID     string `json:"taskId"`
	WorkflowID string `json:"workflowId"`
	ProjectID  string `json:"projectId"`
	AppID      string `json:"appId"`
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
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
	group := w.consumerGroup()
	consumer := w.consumerName()
	klog.Infof("worker reading stream: %s, group: %s, consumer: %s", w.dispatchTopic(), group, consumer)
	// Ensure consumer group exists once on worker start to avoid per-read overhead.
	if err := w.Queue.EnsureGroup(ctx, group); err != nil {
		klog.V(4).Infof("ensure group error: %v", err)
	}
	staleTicker := time.NewTicker(w.workerStaleInterval())
	defer staleTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-staleTicker.C:
			// periodically claim stale pending messages
			mags, err := w.Queue.AutoClaim(ctx, group, consumer, w.workerAutoClaimMinIdle(), w.workerAutoClaimCount())
			if err != nil {
				klog.V(4).Infof("auto-claim error: %v", err)
				continue
			}
			for _, m := range mags {
				if ack, taskID := w.processDispatchMessage(ctx, m); ack {
					klog.Infof("consumer=%s acked message id=%s task=%s", consumer, m.ID, taskID)
					_ = w.Queue.Ack(ctx, group, m.ID)
				} else {
					klog.Warningf("consumer=%s left message pending id=%s task=%s due to processing error", consumer, m.ID, taskID)
				}
			}
		default:
			mags, err := w.Queue.ReadGroup(ctx, group, consumer, w.workerReadCount(), w.workerReadBlock())
			if err != nil {
				klog.V(4).Infof("read group error: %v", err)
				continue
			}
			for _, m := range mags {
				if ack, taskID := w.processDispatchMessage(ctx, m); ack {
					klog.Infof("consumer=%s acked claimed message id=%s task=%s", consumer, m.ID, taskID)
					_ = w.Queue.Ack(ctx, group, m.ID)
				} else {
					klog.Warningf("consumer=%s left claimed message pending id=%s task=%s due to processing error", consumer, m.ID, taskID)
				}
			}
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

func (w *Workflow) processDispatchMessage(ctx context.Context, m msg.Message) (bool, string) {
	td, err := UnmarshalTaskDispatch(m.Payload)
	if err != nil {
		klog.Errorf("decode dispatch failed: %v", err)
		return true, ""
	}
	task, err := repository.TaskByID(ctx, w.Store, td.TaskID)
	if err != nil {
		klog.Errorf("load task %s failed: %v", td.TaskID, err)
		return true, td.TaskID
	}
	if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
		klog.Errorf("run task %s failed: %v", td.TaskID, err)
		return false, td.TaskID
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
