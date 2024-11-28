package service

type ApplicationService interface {
}

type ApplicationServiceImpl struct {
}

func NewApplicationService() ApplicationService {
	return &ApplicationServiceImpl{}
}
