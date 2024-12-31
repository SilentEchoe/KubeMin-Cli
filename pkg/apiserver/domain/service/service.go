package service

import "KubeMin-Cli/pkg/apiserver/config"

// InitServiceBean init all service instance
func InitServiceBean(c config.Config) []interface{} {
	// TODO 1.app
	applicationService := NewApplicationService()
	// TODO 2.clusterService
	
	return []interface{}{
		applicationService,
	}
}
