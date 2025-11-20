package v1

import (
	"encoding/json"
	"fmt"

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

// ConvertWorkflowModelToDTO converts the workflow model into an API-friendly structure.
func ConvertWorkflowModelToDTO(workflow *model.Workflow) (*apisv1.ApplicationWorkflow, error) {
	if workflow == nil {
		return nil, nil
	}
	steps, err := convertWorkflowSteps(workflow.Steps)
	if err != nil {
		return nil, fmt.Errorf("convert workflow %s steps: %w", workflow.ID, err)
	}
	return &apisv1.ApplicationWorkflow{
		ID:           workflow.ID,
		Name:         workflow.Name,
		Alias:        workflow.Alias,
		Namespace:    workflow.Namespace,
		ProjectID:    workflow.ProjectID,
		Description:  workflow.Description,
		Status:       string(workflow.Status),
		Disabled:     workflow.Disabled,
		Steps:        steps,
		CreateTime:   workflow.CreateTime,
		UpdateTime:   workflow.UpdateTime,
		WorkflowType: workflow.WorkflowType,
	}, nil
}

func convertWorkflowSteps(raw *model.JSONStruct) ([]apisv1.WorkflowStepDetail, error) {
	if raw == nil {
		return nil, nil
	}
	var steps model.WorkflowSteps
	if err := json.Unmarshal([]byte(raw.JSON()), &steps); err != nil {
		return nil, err
	}
	result := make([]apisv1.WorkflowStepDetail, 0, len(steps.Steps))
	for _, step := range steps.Steps {
		if step == nil {
			continue
		}
		detail := apisv1.WorkflowStepDetail{
			Name:         step.Name,
			WorkflowType: step.WorkflowType,
			Mode:         step.Mode,
			Components:   flattenPolicies(step.Properties),
		}
		if len(step.SubSteps) > 0 {
			subDetails := make([]apisv1.WorkflowSubStepDetail, 0, len(step.SubSteps))
			for _, sub := range step.SubSteps {
				if sub == nil {
					continue
				}
				subDetails = append(subDetails, apisv1.WorkflowSubStepDetail{
					Name:         sub.Name,
					WorkflowType: sub.WorkflowType,
					Components:   flattenPolicies(sub.Properties),
				})
			}
			detail.SubSteps = subDetails
		}
		result = append(result, detail)
	}
	return result, nil
}

func flattenPolicies(policies []model.Policies) []string {
	if len(policies) == 0 {
		return nil
	}
	var components []string
	for _, policy := range policies {
		if len(policy.Policies) == 0 {
			continue
		}
		components = append(components, policy.Policies...)
	}
	if len(components) == 0 {
		return nil
	}
	return components
}
