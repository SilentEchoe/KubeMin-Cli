package models

type notice struct {
	Model

	ID string `json:"Id"`
	cnTitle string `json:"cnTitle"`
	enTitle string `json:"enTitle"`
	cnContent string `json:"cnContent"`
	enContent string `json:"enContent"`
	publishState int `json:"state"`
}

func GetNotices(pageNum int, pageSize int, maps interface {}) (notices []notice) {
	db.Where(maps).Offset(pageNum).Limit(pageSize).Find(&notices)

	return
}
