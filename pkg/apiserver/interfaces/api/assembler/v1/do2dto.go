package v1

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
)

// ConvertAppModelToBase assemble the Application model to DTO
func ConvertAppModelToBase(app *model.Applications, workflowID string) *apisv1.ApplicationBase {
	appBase := &apisv1.ApplicationBase{
		ID:          app.ID,
		Name:        app.Name,
		Project:     app.Project,
		Alias:       app.Alias,
		CreateTime:  app.CreateTime,
		UpdateTime:  app.UpdateTime,
		Description: app.Description,
		Icon:        app.Icon,
		WorkflowID:  workflowID,
	}
	return appBase
}
