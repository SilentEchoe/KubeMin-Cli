package job

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/infrastructure/locker"
	"kubemin-cli/pkg/apiserver/utils/cache"
)

var (
	shareLocker     locker.Locker
	shareLockerOnce sync.Once
)

func shareInfoFromLabels(labels map[string]string) (string, config.ShareStrategy) {
	if labels == nil {
		return "", ""
	}
	name := strings.TrimSpace(labels[config.LabelShareName])
	if name == "" {
		return "", ""
	}
	rawStrategy := strings.TrimSpace(labels[config.LabelShareStrategy])
	strategy, _ := config.NormalizeShareStrategy(rawStrategy)
	return name, strategy
}

func shareListOptions(name string) metav1.ListOptions {
	selector := labels.Set{config.LabelShareName: name}.String()
	return metav1.ListOptions{LabelSelector: selector}
}

func hasSharedResources(ctx context.Context, name string, listFn func(context.Context, metav1.ListOptions) (int, error)) (bool, error) {
	if name == "" {
		return false, nil
	}
	count, err := listFn(ctx, shareListOptions(name))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func shareLockKey(kind config.ResourceKind, name string) string {
	if kind == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("kubemin-share:%s:%s", kind, name)
}

func acquireShareLock(ctx context.Context, key string) (func(), error) {
	if key == "" {
		return nil, nil
	}
	locker := shareLockerInstance()
	mutex := locker.NewMutex(key)
	if err := mutex.Lock(ctx); err != nil {
		return nil, err
	}
	return func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mutex.Unlock(unlockCtx); err != nil {
			klog.Warningf("failed to release share lock %s: %v", key, err)
		}
	}, nil
}

func shareLockerInstance() locker.Locker {
	shareLockerOnce.Do(func() {
		shareLocker = initShareLocker()
	})
	if shareLocker == nil {
		shareLocker = locker.NewNoopLocker("kubemin-share")
	}
	return shareLocker
}

func initShareLocker() locker.Locker {
	redisClient := cache.GetGlobalRedisClient()
	if redisClient != nil {
		redisLocker, err := locker.New(locker.Config{
			Type:        locker.TypeRedis,
			RedisClient: redisClient,
			Prefix:      "kubemin-share",
		})
		if err == nil {
			return redisLocker
		}
		klog.Warningf("failed to init redis locker, falling back to memory: %v", err)
	}
	memLocker, err := locker.New(locker.Config{Type: locker.TypeMemory, Prefix: "kubemin-share"})
	if err != nil {
		klog.Warningf("failed to init memory locker, using noop: %v", err)
		return locker.NewNoopLocker("kubemin-share")
	}
	return memLocker
}

func resolveSharedResource(ctx context.Context, name string, strategy config.ShareStrategy, kind config.ResourceKind, listFn func(context.Context, metav1.ListOptions) (int, error)) (func(), bool, error) {
	if name == "" || strategy == "" {
		return nil, false, nil
	}
	if strategy == config.ShareStrategyIgnore {
		return nil, true, nil
	}
	if strategy != config.ShareStrategyDefault {
		return nil, false, nil
	}
	unlock, err := acquireShareLock(ctx, shareLockKey(kind, name))
	if err != nil {
		return nil, false, err
	}
	release := func() {
		if unlock != nil {
			unlock()
		}
	}
	exists, err := hasSharedResources(ctx, name, listFn)
	if err != nil {
		release()
		return nil, false, err
	}
	if exists {
		release()
		return nil, true, nil
	}
	return unlock, false, nil
}
