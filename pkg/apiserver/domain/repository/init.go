package repository

// InitRepositoryBean initializes all repository instances.
// Dependencies are injected via struct tags by the IoC container.
func InitRepositoryBean() []interface{} {
	return []interface{}{
		NewApplicationRepository(),
		NewWorkflowRepository(),
		NewComponentRepository(),
		NewWorkflowQueueRepository(),
	}
}

