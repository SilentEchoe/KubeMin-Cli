package models

type sysUser struct {
	ID int `gorm:"primary_key" json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func CheckAuth(username, password string) bool {
	var sysuser sysUser
	db.Select("id").Where(sysUser{Username : username, Password : password}).First(&sysuser)
	if sysuser.ID > 0 {
		return true
	}
	return false
}