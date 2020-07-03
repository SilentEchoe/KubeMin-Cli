package models

type ConfigFiles struct {
	Model

	ID             int    `json:"id"`
	ConfigFileName string `json:"config_file_name"`
	ConfigType     string `json:"config_type"`
	PowerDelay     string `json:"power_delay"`
	PageWriteDelay string `json:"page_write_delay"`
	ConfigPassword string `json:"config_password"`
	PageWriteByte  string `json:"page_write_byte"`
	PreOperation   string `json:"pre_operation"`
	IsDelete       int    `json:"isdelete"`
}

type ConfigFileManger struct {
	Model

	ID             int    `json:"id"`
	ModalId        int    `json:"modal_id"`
	CompatibleType string `json:"compatible_type"`
	ConfigFiles    string `json:"config_files"`
	Enable         string `json:"enable"`
}

// 查询对应的配置文件
func GetConfigFileById(id []int) (confides []ConfigFiles) {
	db.Where("id IN (?)", id).Find(&confides)
	return
}

// GetConfigsById 根据型号id查找配置文件
// confideManger.ConfigFiles 返回参数  返回配置文件id组
// string 返回类型
func GetConfigsById(id int) string {
	var confideManger ConfigFileManger
	db.Where("modal_id = ?", id).First(&confideManger)
	return confideManger.ConfigFiles
}
