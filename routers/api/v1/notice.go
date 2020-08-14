package v1

import (
	"LearningNotes-GoMicro/pkg/app"
	"LearningNotes-GoMicro/pkg/setting"
	"LearningNotes-GoMicro/pkg/util"
	"LearningNotes-GoMicro/service/notice_service"
	"github.com/astaxie/beego/validation"
	"net/http"

	"github.com/gin-gonic/gin"

	"LearningNotes-GoMicro/pkg/e"

	"LearningNotes-GoMicro/models"
)


// @summary 获取全部通知
// @produce  json
// @param name query string true "name"
// @param state query int false "state"
// @param created_by query int false "createdby"
// @success 200 {string} string "{"code":200,"data":{},"msg":"ok"}"
// @router /api/v1/tags [post]
func GetNoticesPage(c *gin.Context) {
	appG := app.Gin{c}
	maps := make(map[string]interface{})
	tags, err := models.GetNoticePage(util.GetPage(c),setting.AppSetting.PageSize,maps)
	if err != nil {
		appG.Response(http.StatusInternalServerError, e.ERROR_GET_TAGS_FAIL, nil)
		return
	}
	appG.Response(http.StatusOK, e.SUCCESS, map[string]interface{}{
		"lists": tags,
	})
}

// @summary 获取全部通知
// @produce  json
// @param name query string true "name"
// @param state query int false "state"
// @param created_by query int false "createdby"
// @success 200 {string} string "{"code":200,"data":{},"msg":"ok"}"
// @router /api/v1/tags [post]
func GetNoticesPageTest(c *gin.Context) {
	appG := app.Gin{c}
	maps := make(map[string]interface{})
	tags, err := models.GetNoticePageTest(util.GetPage(c),setting.AppSetting.PageSize,maps)
	println(tags)
	if err != nil {
		appG.Response(http.StatusInternalServerError, e.ERROR_GET_TAGS_FAIL, nil)
		return
	}
	appG.Response(http.StatusOK, e.SUCCESS, map[string]interface{}{
		"lists": tags,
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

func GetNoticesByRedis(c *gin.Context)  {
	appG := app.Gin{c}
	noticeService := notice_service.Notice{
		PageNum:  util.GetPage(c),
		PageSize: setting.AppSetting.PageSize,
	}

	tags, err := noticeService.GetNoticeAll()
	if err != nil {
		appG.Response(http.StatusInternalServerError, e.ERROR_GET_TAGS_FAIL, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, map[string]interface{}{
		"lists": tags,

	})


}


