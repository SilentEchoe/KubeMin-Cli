package models

type Notice struct {
	Model

	Cntitle    string `gorm:"type:text;column:cnTitle"`
	PublishState int `gorm:"column:publishState"`
}

func GetNotices(pageNum int, pageSize int, maps interface {}) (notices []Notice) {
	db.Where(maps).Offset(pageNum).Limit(pageSize).Find(&notices)

	return
}

func GetNoticeTotal(maps interface {}) (count int){
	db.Model(&Notice{}).Where(maps).Count(&count)

	return
}