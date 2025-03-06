package model

func init() {
	RegisterModel(&Applications{})
}

type Applications struct {
	BaseModel
	Name        string            `json:"name" gorm:"primaryKey"` //应用名称
	Alias       string            `json:"alias"`                  //别名
	Project     string            `json:"project"`                //项目
	Description string            `json:"description"`            //详情
	Icon        string            `json:"icon"`                   //图标
	Labels      map[string]string `json:"labels,omitempty" gorm:"serializer:json"`
	Annotations map[string]string `json:"annotations,omitempty" gorm:"serializer:json"`
}

func (a Applications) PrimaryKey() string {
	//TODO implement me
	panic("implement me")
}

func (a Applications) TableName() string {
	return tableNamePrefix + "applications"
}

func (a Applications) ShortTableName() string {
	return "app"
}

// Index return custom index
func (a *Applications) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if a.Name != "" {
		index["name"] = a.Name
	}
	if a.Project != "" {
		index["project"] = a.Project
	}
	return index
}
