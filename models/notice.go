package models

type Notice struct {
	Model

	Cntitle    string `gorm:"type:text;column:cnTitle"`
	Entitle    string `gorm:"type:text;column:enTitle"`

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

func AddNotices(cntitle string,entitle string ) bool  {
	db.Create(&Notice{
		Cntitle:      cntitle,
		Entitle:	  entitle,
		PublishState: 0,
	})
	return  true
}

func ExistNoticeByID(id int) bool  {
	var notice Notice
	db.Select("id").Where("id = ?", id).First(&notice)
	if notice.ID > 0 {
		return true
	}
	return  false
}

func DeleteNotice(id int) bool  {
	db.Where("id = ?",id).Delete(&Notice{})
	return true
}

func EditNotice(id int,data interface{}) bool {
	db.Model(&Notice{}).Where("id = ?",id).Update(data)
	return  true
}




