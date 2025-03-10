package api

import (
	"encoding/json"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Demo struct {
}

type Response struct {
	Name string `json:"name"` // 确保字段可导出且带JSON标签
}

func (d *Demo) GetWebServiceRoute() *restful.WebService {
	return nil
}

func NewDemo() NewInterface {
	return &Demo{}
}

func (d *Demo) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/list", d.ListDemos)
}

func (d *Demo) ListDemos(c *gin.Context) {
	fmt.Println("[Debug] 进入处理函数") // 确认是否执行

	data := Response{Name: "demo"}
	fmt.Printf("[Debug] 响应数据：%+v\n", data) // 打印数据

	// 检查JSON序列化结果
	jsonData, _ := json.Marshal(data)
	fmt.Printf("[Debug] JSON序列化结果：%s\n", jsonData)

	c.JSON(http.StatusOK, data)
}
