package router

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
)

// scoreUploadLimit 是成绩导入 Excel 的大小上限（10MB）。
const scoreUploadLimit = 10 << 20

// parseScoreImportFile 解析成绩导入的 multipart 表单，返回结构化成绩行。
//
// excelize 需要 Reader，文件读入内存后立即解析；请求体整体限流，只接受 .xlsx。
func parseScoreImportFile(c *gin.Context) ([]model.ScorePreviewInputRow, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, scoreUploadLimit)
	if err := c.Request.ParseMultipartForm(scoreUploadLimit); err != nil {
		return nil, httpx.ErrBadRequest("表单解析失败：请上传 ≤10MB 的 .xlsx 成绩文件")
	}
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()

	file, err := c.FormFile("file")
	if err != nil {
		return nil, httpx.ErrBadRequest("请上传成绩 Excel 文件（.xlsx）")
	}
	if file.Size <= 0 || file.Size > scoreUploadLimit {
		return nil, httpx.ErrBadRequest("Excel 文件为空或超过 10MB")
	}
	if strings.ToLower(filepath.Ext(file.Filename)) != ".xlsx" {
		return nil, httpx.ErrBadRequest("仅支持 .xlsx 格式的成绩文件")
	}
	r, err := file.Open()
	if err != nil {
		return nil, httpx.ErrBadRequest("读取上传文件失败").WithCause(err)
	}
	defer func() { _ = r.Close() }()

	return service.ParseScoreWorkbook(r)
}
