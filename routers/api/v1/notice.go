package v1

import (
	"github.com/astaxie/beego/validation"
	"net/http"

	"github.com/gin-gonic/gin"
	//"github.com/astaxie/beego/validation"
	"github.com/unknwon/com"

	"LearningNotes-GoMicro/pkg/e"

	"LearningNotes-GoMicro/models"

	"LearningNotes-GoMicro/pkg/util"

	"LearningNotes-GoMicro/pkg/setting"
)


// @Summary 获取全部通知
// @Produce  json
// @Param name query string true "Name"
// @Param state query int false "State"
// @Param created_by query int false "CreatedBy"
// @Success 200 {string} string "{"code":200,"data":{},"msg":"ok"}"
// @Router /api/v1/tags [post]
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

	data["lists"] = models.GetNotices(util.GetPage(c), setting.AppSetting.PageSize, maps)
	data["total"] = models.GetNoticeTotal(maps)

	c.JSON(http.StatusOK, gin.H{
		"code" : code,
		"msg" : e.GetMsg(code),
		"data" : data,
	})
}

// 新增通知
func AddNotices(c *gin.Context)  {

	cnTitle := c.PostForm("cnTitle")
	enTitle := c.PostForm("enTitle")
	//state := com.StrTo(c.DefaultPostForm("state", "0")).MustInt()
	valid := validation.Validation{}
	valid.Required(cnTitle, "cnTitle").Message("公告标题不能为空")
	code := e.INVALID_PARAMS

	if ! valid.HasErrors() {
		code = e.SUCCESS
		models.AddNotices(cnTitle,enTitle)
	}

	c.JSON(http.StatusOK, gin.H{
		"code" : code,
		"msg" : e.GetMsg(code),
		"data" : make(map[string]string),
	})



}

func EditNotice(c *gin.Context)  {
	
}

func DeleteNoticeByID(c *gin.Context)  {
	
}