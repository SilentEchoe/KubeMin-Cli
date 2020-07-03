package models

type ModalBinType struct {
	Model

	MadalenaId    int           `json:"modal_id" gorm:"index"`
	ConfModalName ConfModalName `json:"ConfModalName"`

	CompatibleType string `json:"compatible_type"`
	Type           string `json:"type"`

	//处理方式 0使用bin模板 1使用bin文件
	ProcessingMethod string `json:"processing_method"`
	ThresholdValue   int    `json:"threshold_value"`

	State int `json:"model_type"`
}

func GetModelTypes(pageNum int, pageSize int, maps interface{}) (madalenaType []ModalBinType) {
	db.Preload("ConfModalName").Where(maps).Offset(pageNum).Limit(pageSize).Find(&madalenaType)

	return
}

func GetModelTypeTotal(maps interface{}) (count int) {
	db.Model(&ModalBinType{}).Where(maps).Count(&count)

	return
}

func GetModelTypeId(modelId int, compatibleType string) (count int) {
	var madalena ModalBinType
	db.Where(&ModalBinType{CompatibleType: compatibleType, MadalenaId: modelId}).First(&madalena)

	return madalena.ID
}

// 查询bin模板
func GetBinTemplate(maps interface{}) (madalenaType []ModalBinType) {
	db.Where(maps).First(&madalenaType)
	return
}
