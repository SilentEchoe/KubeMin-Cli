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


func (n *Notice) GetNoticeAll() ([]models.Notice, error) {
	var (
		notices, cacheTags []models.Notice
	)

	cache := cache_service.Notice{
		PageNum:  n.PageNum,
		PageSize: n.PageSize,
	}
	key := cache.GetNoticeKey()
	if gredis.Exists(key) {
		data, err := gredis.Get(key)
		if err != nil {
			logging.Info(err)
		} else {
			json.Unmarshal(data, &cacheTags)
			return cacheTags, nil
		}
	}
	notices, err := models.GetAll()
	if err != nil {
		return nil, err
	}

	gredis.Set(key, notices, 3000)
	return notices, nil
}



func (n *Notice) getMaps() map[string]interface{} {
	maps := make(map[string]interface{})
	if n.PublishState != -1 {
		maps["PublishState"] = n.PublishState
	}

	return maps
}
