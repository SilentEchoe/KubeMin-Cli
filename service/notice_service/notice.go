package notice_service

import (
	"encoding/json"

	"LearningNotes-GoMicro/models"
	"LearningNotes-GoMicro/pkg/gredis"
	"LearningNotes-GoMicro/pkg/logging"
	"LearningNotes-GoMicro/service/cache_service"
)

type Notice struct {
	ID            int
	Cntitle    string
	Entitle    string
	PublishState int

	PageNum  int
	PageSize int
}

func (a *Notice) GetAll() ([]*models.Notice, error) {
	var (
		notices, cacheNotices []*models.Notice
	)
	cache := cache_service.Notice{
		Cntitle : a.Cntitle,
		Entitle : a.Entitle,

		PageNum:  a.PageNum,
		PageSize: a.PageSize,
	}
	key := cache.GetArticlesKey()
	if gredis.Exists(key) {
		data, err := gredis.Get(key)
		if err != nil {
			logging.Info(err)
		} else {
			json.Unmarshal(data, &cacheNotices)
			return cacheNotices, nil
		}
	}

	notices, err := models.GetNotice(a.PageNum, a.PageSize, a.getMaps())
	if err != nil {
		return nil, err
	}

	gredis.Set(key, notices, 3600)
	return notices, nil
}



func (a *Notice) getMaps() map[string]interface{} {
	maps := make(map[string]interface{})
	if a.PublishState != -1 {
		maps["PublishState"] = a.PublishState
	}

	return maps
}
