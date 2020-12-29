package logic

import (
	"context"

	"shorturl/internal/svc"
	"shorturl/internal/types"

	"github.com/tal-tech/go-zero/core/logx"
)

type ShortenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewShortenLogic(ctx context.Context, svcCtx *svc.ServiceContext) ShortenLogic {
	return ShortenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ShortenLogic) Shorten(req types.ShortenReq) (*types.ShortenResp, error) {
	// todo: add your logic here and delete this line

	return &types.ShortenResp{}, nil
}
