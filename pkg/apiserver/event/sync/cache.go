package sync

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"errors"
	"strings"
)

type cached struct {
	revision string
	targets  int64
}

// initCache will initialize the cache
func (c *CR2UX) initCache(ctx context.Context) error {
	appsRaw, err := c.ds.List(ctx, &model.Applications{}, &datastore.ListOptions{})
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil
		}
		return err
	}
	for _, appR := range appsRaw {
		app, ok := appR.(*model.Applications)
		if !ok {
			continue
		}
		//描述同步的修订名称
		revision, ok := app.Labels[model.LabelSyncRevision]
		if !ok {
			continue
		}
		// 从lab的标签获取命名空间
		namespace := app.Labels[model.LabelSyncNamespace]
		var key = formatAppComposedName(app.Name, namespace)
		if strings.HasSuffix(app.Name, namespace) {
			key = app.Name
		}
		// 如果从应用状态同步，应该先检查目标
		c.syncCache(key, revision, 0)
	}
	return nil
}

func (c *CR2UX) syncCache(key string, revision string, targets int64) {
	// update cache
	c.cache.Store(key, &cached{revision: revision, targets: targets})
}
