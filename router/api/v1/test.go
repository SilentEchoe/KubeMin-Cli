package v1

import (
	"LearningNotes-GoMicro/pkg/e"

	"LearningNotes-GoMicro/Helper"
	"LearningNotes-GoMicro/ProdService"
	"github.com/gin-gonic/gin"
	"net/http"
)

// @Summary 测试接口
// @Produce  json
// @Param name query int true "modelId"
// @Success 200 {string} string "{"code":200,"data":{},"msg":"ok"}"
// @Router /api/v1/GetConfigFiles [Get]
func GetTestList (c *gin.Context)  {

	data := make(map[string]interface{})
	//maps := make(map[string]interface{})
	code := e.ERROR
	var pr Helper.ProdsRequest
	err := c.Bind(&pr)
	if err != nil || pr.Size <=0 {
		pr= Helper.ProdsRequest{Size:2}
	}
	code = e.SUCCESS
	ProdService.NewProdList(pr.Size)
	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg":  e.GetMsg(code),
		"data": data,
	})


}