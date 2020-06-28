package ServiceImpl

import (
	Service "LearningNotes-GoMicro/Services"
	"context"
	_ "LearningNotes-GoMicro/Services"
)

type  ProdService struct {

}


func (*ProdService) GetProdsList(ctx context.Context,int *Service.ProdsRequest,res *Service.ProdListResponse) error{
	models := make([]*Service.pordModel,0)
}