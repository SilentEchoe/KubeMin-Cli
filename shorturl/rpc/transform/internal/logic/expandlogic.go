package logic

import (
	"context"

	"shorturl/rpc/transform/internal/svc"
	"shorturl/rpc/transform/transform"

	"github.com/tal-tech/go-zero/core/logx"
)

type ExpandLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExpandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExpandLogic {
	return &ExpandLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExpandLogic) Expand(in *transform.ExpandReq) (*transform.ExpandResp, error) {
	// todo: add your logic here and delete this line

	return &transform.ExpandResp{}, nil
}
