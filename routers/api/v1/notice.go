package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	//"github.com/astaxie/beego/validation"
	"github.com/unknwon/com"

	"LearningNotes-GoMicro/pkg/e"

	"LearningNotes-GoMicro/models"

	"LearningNotes-GoMicro/pkg/util"

	"LearningNotes-GoMicro/pkg/setting"
)

//获取全部通知
func GetNotices(c *gin.Context) {
	name := c.Query("name")

	maps := make(map[string]interface{})
	data := make(map[string]interface{})

	if name != "" {
		maps["name"] = name
	}

	var state int = -1
	if arg := c.Query("state"); arg != "" {
		state = com.StrTo(arg).MustInt()
		maps["state"] = state
	}

	code := e.SUCCESS

	data["lists"] = models.GetNotices(util.GetPage(c), setting.PageSize, maps)
	data["total"] = models.GetNoticeTotal(maps)

	c.JSON(http.StatusOK, gin.H{
		"code" : code,
		"msg" : e.GetMsg(code),
		"data" : data,
	})
}

// 新增通知
func AddNotices(c *gin.Context)  {

	data := make(map[string]interface{})
	name := c.PostForm("name")

	data["lists"] =name

	code := e.SUCCESS
	c.JSON(http.StatusOK, gin.H{
		"code" : code,
		"msg" : e.GetMsg(code),
		"data" : data,
	})



}