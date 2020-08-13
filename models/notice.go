package models

import "github.com/jinzhu/gorm"

type Notice struct {
	Model

	Cntitle    string `gorm:"type:text;column:cnTitle"`
	Entitle    string `gorm:"type:text;column:enTitle"`

	PublishState int `gorm:"column:publishState"`
}

// 获取全部的通告信息
func GetAll() ([]Notice,error) {
	var (
		notice []Notice
		err  error
	)
	err = db.Find(&notice).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return notice, nil
}


func GetNoticePage(pageNum int, pageSize int, maps interface{}) ([]Notice, error) {
	var (
		tags []Notice
		err  error
	)

	if pageSize > 0 && pageNum > 0 {
		err = db.Where(maps).Find(&tags).Offset(pageNum).Limit(pageSize).Error
	} else {
		err = db.Where(maps).Find(&tags).Error
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return tags, nil


}

func GetNoticePageTest(pageNum int, pageSize int, maps interface{}) ([]Notice, error)  {
	var (
		tags []Notice
		err  error
	)

	/*if pageSize > 0 && pageNum > 0 {
		 db.Where(maps).Find(&tags).Offset(pageNum).Limit(pageSize)
	} else {
		err = db.Where(maps).Find(&tags).Offset(0).Limit(10).Error
	}*/
	db.Where(maps).Find(&tags).Offset(pageNum).Limit(pageSize)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return tags, nil
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






