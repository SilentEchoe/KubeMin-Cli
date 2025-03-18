package service

import "KubeMin-Cli/pkg/apiserver/config"

// InitServiceBean init all service instance
func InitServiceBean(c config.Config) []interface{} {
	applicationService := NewApplicationService()
	workflowService := NewWorkflowService()

	return []interface{}{
		applicationService,
		workflowService,
	}
}
