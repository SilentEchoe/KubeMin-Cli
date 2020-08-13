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

/*func (a *Notice) Add() error {
	notice := map[string]interface{}{
		"Cntitle":           a.Cntitle,
		"Entitle":           a.Entitle,
		"PublishState":      a.PublishState,

	}

	if err := models.AddNotices(notice); err != nil {
		return err
	}

	return nil
}*/



func (a *Notice) Get() (*models.Notice, error) {
	var cacheArticle *models.Notice

	cache := cache_service.Notice{ID: a.ID}
	key := cache.GetArticleKey()
	if gredis.Exists(key) {
		data, err := gredis.Get(key)
		if err != nil {
			logging.Info(err)
		} else {
			json.Unmarshal(data, &cacheArticle)
			return cacheArticle, nil
		}
	}

	article, err := models.GetNotices()
	if err != nil {
		return nil, err
	}

	gredis.Set(key, article, 3600)
	return article, nil
}

func (a *Article) GetAll() ([]*models.Article, error) {
	var (
		articles, cacheArticles []*models.Article
	)

	cache := cache_service.Article{
		TagID: a.TagID,
		State: a.State,

		PageNum:  a.PageNum,
		PageSize: a.PageSize,
	}
	key := cache.GetArticlesKey()
	if gredis.Exists(key) {
		data, err := gredis.Get(key)
		if err != nil {
			logging.Info(err)
		} else {
			json.Unmarshal(data, &cacheArticles)
			return cacheArticles, nil
		}
	}

	articles, err := models.GetArticles(a.PageNum, a.PageSize, a.getMaps())
	if err != nil {
		return nil, err
	}

	gredis.Set(key, articles, 3600)
	return articles, nil
}

func (a *Article) Delete() error {
	return models.DeleteArticle(a.ID)
}

func (a *Article) ExistByID() (bool, error) {
	return models.ExistArticleByID(a.ID)
}

func (a *Article) Count() (int, error) {
	return models.GetArticleTotal(a.getMaps())
}

func (a *Notice) getMaps() map[string]interface{} {
	maps := make(map[string]interface{})
	maps["deleted_on"] = 0
	if a.PublishState != -1 {
		maps["state"] = a.PublishState
	}

	return maps
}
