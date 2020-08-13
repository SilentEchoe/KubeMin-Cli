package cache_service

import (
	"strconv"
	"strings"

	"LearningNotes-GoMicro/pkg/e"
)

type Notice struct {
	ID    int
	Cntitle string
	Entitle string

	PageNum  int
	PageSize int
}

func (a *Notice) GetArticleKey() string {
	return e.CACHE_ARTICLE + "_" + strconv.Itoa(a.ID)
}

func (a *Notice) GetArticlesKey() string {
	keys := []string{
		e.CACHE_ARTICLE,
		"LIST",
	}

	if a.ID > 0 {
		keys = append(keys, strconv.Itoa(a.ID))
	}
	if a.PageNum > 0 {
		keys = append(keys, strconv.Itoa(a.PageNum))
	}
	if a.PageSize > 0 {
		keys = append(keys, strconv.Itoa(a.PageSize))
	}

	return strings.Join(keys, "_")
}