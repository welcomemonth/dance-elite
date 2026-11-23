package router

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// lotteryUploadLimit 是报名 Excel 上传大小上限（30MB，报名表通常 <2MB）。
const lotteryUploadLimit = 30 << 20

// parseLotteryImportForm 解析后台抽签导入的 multipart 表单。
//
// Excel 读入内存后交给 service（excelize 需要 Reader），请求体整体限流，
// 避免超大文件占满内存；只接受 .xlsx（excelize 不支持旧版 .xls）。
func parseLotteryImportForm(c *gin.Context) (service.AdminLotteryImportInput, error) {
	var input service.AdminLotteryImportInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, lotteryUploadLimit)
	if err := c.Request.ParseMultipartForm(lotteryUploadLimit); err != nil {
		return input, httpx.ErrBadRequest("表单解析失败：请上传 ≤30MB 的 .xlsx 报名表")
	}

	if raw := c.PostForm("event_id"); raw != "" {
		eventID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || eventID < 0 {
			return input, httpx.ErrBadRequest("赛事 ID 非法")
		}
		input.EventID = eventID
	}
	input.Title = c.PostForm("title")
	input.Source = c.PostForm("source")
	input.StartAt = c.PostForm("start_at")
	input.EndAt = c.PostForm("end_at")
	input.SMSRemind = c.PostForm("sms_remind_template")
	input.SMSResult = c.PostForm("sms_result_template")
	input.Replace = parseFormBool(c.PostForm("replace"))
	if raw := c.PostForm("remind_before_minutes"); raw != "" {
		remind, err := strconv.Atoi(raw)
		if err != nil || remind < 0 {
			return input, httpx.ErrBadRequest("提醒提前分钟数不合法")
		}
		input.Remind = remind
	}

	// 从赛事报名生成时不需要上传 Excel。
	if input.Source == "event" {
		return input, nil
	}
	file, err := c.FormFile("file")
	if err != nil {
		return input, httpx.ErrBadRequest("请上传报名 Excel 文件（.xlsx）")
	}
	if file.Size <= 0 || file.Size > lotteryUploadLimit {
		return input, httpx.ErrBadRequest("Excel 文件为空或超过 30MB")
	}
	opened, err := file.Open()
	if err != nil {
		return input, httpx.ErrBadRequest("读取上传文件失败，请重试")
	}
	defer opened.Close()
	content := make([]byte, file.Size)
	if _, err := io.ReadFull(opened, content); err != nil {
		return input, httpx.ErrBadRequest("读取上传文件失败，请重试")
	}
	input.Excel = bytes.NewReader(content)
	return input, nil
}

// parseFormBool 解析可选布尔表单字段，非法值按 false 处理。
func parseFormBool(raw string) bool {
	value, err := strconv.ParseBool(raw)
	return err == nil && value
}
