package service

import "context"

// IOC 实现的具体服务

type IApplications interface {
	Get(ctx context.Context) string
}

var _ IApplications = (*ApplicationService)(nil)

type ApplicationService struct{}

func (a *ApplicationService) Get(ctx context.Context) string {
	return "application"
}

func NewApplicationService() IApplications {
	return &ApplicationService{}
}
