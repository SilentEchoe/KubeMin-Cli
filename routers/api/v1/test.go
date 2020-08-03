package v1

import (
	//"LearningNotes-GoMicro/pkg/e"
	"github.com/gin-gonic/gin"
	//"net/http"
)

// @Summary 测试接口
// @Produce  json
// @Param name query int true "modelId"
// @Success 200 {string} string "{"code":200,"data":{},"msg":"ok"}"
// @Router /api/v1/GetConfigFiles [Get]
func GetTestList(c *gin.Context) {

	data := make(map[string]interface{})

	//code = e.SUCCESS
	data["list"] = "1"

	c.JSON(200,
		gin.H{
			"data": 2})

	/*c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg":  e.GetMsg(code),
		"data": ProdService.NewProdList(pr.Size),
	})*/

}
