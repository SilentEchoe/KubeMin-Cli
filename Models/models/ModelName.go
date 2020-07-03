package models

type ConfModalName struct {
	Model

	ModuleName string `json:"modal_name"`
	CreatedBy  string `json:"created_by"`
	ModifiedBy string `json:"modified_by"`
	State      int    `json:"is_start"`
	ParentId   int    `json:"parent_id"`
}

func GetModelNames(pageNum int, pageSize int, maps interface{}) (modelNames []ConfModalName) {
	db.Where(maps).Offset(pageNum).Limit(pageSize).Find(&modelNames)

	return
}

func GetModelNameTotal(maps interface{}) (count int) {
	db.Model(&ConfModalName{}).Where(maps).Count(&count)

	return
}

func ExistModelNameByName(name string) bool {
	var modelName ConfModalName
	db.Select("id").Where("module_name = ?", name).First(&modelName)
	if modelName.ID > 0 {
		return true
	}

	return false
}

func AddModelName(name string, state int, createdBy string, parentId int) bool {
	db.Create(&ConfModalName{
		ModuleName: name,
		State:      state,
		CreatedBy:  createdBy,
		ParentId:   parentId,
	})

	return true
}

func ExistModelNameByID(id int) bool {
	var modelName ConfModalName
	db.Select("id").Where("id = ?", id).First(&modelName)
	if modelName.ID > 0 {
		return true
	}
	return false
}

func DeleteModelName(id int) bool {
	db.Where("id = ?", id).Delete(&ConfModalName{})

	return true
}

func EditModelName(id int, data interface{}) bool {
	db.Model(&ConfModalName{}).Where("id = ?", id).Updates(data)

	return true
}
