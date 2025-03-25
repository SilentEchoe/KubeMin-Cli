package v1

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
)

// ConvertAppModelToBase assemble the Application model to DTO
func ConvertAppModelToBase(app *model.Applications) *apisv1.ApplicationBase {
	appBase := &apisv1.ApplicationBase{
		ID:          app.ID,
		Name:        app.Name,
		Alias:       app.Alias,
		CreateTime:  app.CreateTime,
		UpdateTime:  app.UpdateTime,
		Description: app.Description,
		Icon:        app.Icon,
	}
	return appBase
}
