package models

type notice struct {
	Model

	Cntitle    string `gorm:"type:text;column:cnTitle"`
	PublishState int `gorm:"type:text;column:publishState";json:"publishState"`
}

func GetNotices(pageNum int, pageSize int, maps interface {}) (notices []notice) {
	db.Where(maps).Offset(pageNum).Limit(pageSize).Find(&notices)

	return
}
