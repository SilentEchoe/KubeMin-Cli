package Weblib

import (
	"LearningNotes-GoMicro/Services"

	"github.com/gin-gonic/gin"

	"fmt"

	"context"
)

func GetProdsList(ginCtx *gin.Context)  {
	prodService:=ginCtx.Keys["prodservice"].(Services.ProdService)

	var prodReq Services.ProdsRequest
	err := ginCtx.Bind(&prodReq)
	if err !=nil {
		fmt.Println(err)
		ginCtx.JSON(500,gin.H{"status":err.Error()})
	}else {
		prodRes,_ := prodService.GetProdsList(context.Background(),&prodReq)

		ginCtx.JSON(200,gin.H{"data":prodRes.Data})
	}
}