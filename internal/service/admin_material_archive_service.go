package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/storage"
	"github.com/meetmonth/dance-elite/internal/store"
)

// 媒体归档审计动作（admin_logs.action）。
const adminActionMaterialArchive = "admin.material.archive"

// materialArchiveMaxPlayers 限制单次按选手筛选的条目数，防止请求体过大。
const materialArchiveMaxPlayers = 500

// MaterialArchiveService 是赛事媒体资源归档导出服务（S5-1/S5-2，API 文档 12.6）：
// 按选手分目录把证件照/音频流式打包为 zip，禁止把文件整体读入内存。
type MaterialArchiveService struct {
	stores *store.Store
	files  *storage.Store
	now    func() time.Time
}

// NewMaterialArchiveService 构造媒体归档服务。
func NewMaterialArchiveService(stores *store.Store, files *storage.Store) *MaterialArchiveService {
	return &MaterialArchiveService{stores: stores, files: files, now: time.Now}
}

// MaterialArchiveOptions 是归档筛选条件。
type MaterialArchiveOptions struct {
	PlayerNos       []string
	IncludeCanceled bool
}

// MaterialArchiveStats 是归档结果（用于审计：选手数、文件数）。
type MaterialArchiveStats struct {
	Players int
	Files   int
}

// FileName 返回建议的下载文件名。
func (s *MaterialArchiveService) FileName(eventID int64) string {
	return fmt.Sprintf("materials_%d_%s.zip", eventID, s.now().Format("20060102"))
}

// Archive 把赛事媒体流式写入 w，返回统计信息。调用方负责设置下载响应头。
func (s *MaterialArchiveService) Archive(ctx context.Context, adminID, eventID int64,
	opts MaterialArchiveOptions, w io.Writer) (*MaterialArchiveStats, error) {
	if s.files == nil {
		return nil, httpx.ErrInternal(errors.New("storage not configured"), "文件存储未配置")
	}
	event, err := s.stores.Events.Get(ctx, eventID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("赛事不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载赛事失败，请稍后重试")
	}
	playerNos, err := normalizeArchivePlayers(opts.PlayerNos)
	if err != nil {
		return nil, err
	}

	rows, err := s.stores.Registrations.ListMaterialsForArchive(ctx, eventID, playerNos, opts.IncludeCanceled)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载媒体资料失败，请稍后重试")
	}

	zw := zip.NewWriter(w)
	root := archiveRootName(event.Name, event.ID)
	stats := &MaterialArchiveStats{}
	for _, group := range groupArchiveRows(rows) {
		written, err := s.writePlayerGroup(ctx, zw, root, group)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if written > 0 {
			stats.Players++
			stats.Files += written
		}
	}
	if err := zw.Close(); err != nil {
		return nil, httpx.ErrInternal(err, "生成归档文件失败")
	}

	if err := s.auditArchive(ctx, adminID, eventID, opts, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// auditArchive 记录导出操作日志（导出人、赛事、选手数、文件数、筛选条件）。
func (s *MaterialArchiveService) auditArchive(ctx context.Context, adminID, eventID int64,
	opts MaterialArchiveOptions, stats *MaterialArchiveStats) error {
	err := s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		targetType := "event"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionMaterialArchive, &targetType, &eventID,
			map[string]any{
				"players":          stats.Players,
				"files":            stats.Files,
				"include_canceled": opts.IncludeCanceled,
				"player_nos":       opts.PlayerNos,
			})
	})
	if err != nil {
		return httpx.ErrInternal(err, "记录操作日志失败")
	}
	return nil
}

// archivePlayerGroup 是同一选手的归档资料（多舞种时有多条音频）。
type archivePlayerGroup struct {
	name   string
	no     string
	photo  *string
	audios []archiveAudio
}

// archiveAudio 是一条音频及其归属舞种信息（多舞种时用于文件名区分）。
type archiveAudio struct {
	path      string
	program   string
	level     string
	danceType string
}

// groupArchiveRows 按 player_no 归并报名资料，保持数据库返回顺序。
func groupArchiveRows(rows []store.MaterialArchiveRow) []archivePlayerGroup {
	index := make(map[string]int, len(rows))
	groups := make([]archivePlayerGroup, 0, len(rows))
	for _, row := range rows {
		i, ok := index[row.PlayerNo]
		if !ok {
			i = len(groups)
			index[row.PlayerNo] = i
			groups = append(groups, archivePlayerGroup{name: row.PlayerName, no: row.PlayerNo})
		}
		g := &groups[i]
		if g.photo == nil && row.PhotoPath != nil && strings.TrimSpace(*row.PhotoPath) != "" {
			g.photo = row.PhotoPath
		}
		if row.AudioPath != nil && strings.TrimSpace(*row.AudioPath) != "" {
			g.audios = append(g.audios, archiveAudio{
				path:      *row.AudioPath,
				program:   ptrValue(row.ProgramName, ""),
				level:     row.Level,
				danceType: row.DanceType,
			})
		}
	}
	return groups
}

// writePlayerGroup 把一名选手的照片/音频写入 zip，返回实际写入的文件数。
// 单个文件缺失（磁盘已清理）跳过，不视为失败。
func (s *MaterialArchiveService) writePlayerGroup(ctx context.Context, zw *zip.Writer,
	root string, g archivePlayerGroup) (int, error) {
	dir := root + "/" + safeZipName(g.name) + "_" + g.no
	written := 0
	used := map[string]bool{}
	if g.photo != nil {
		name := uniqueZipName(used, "证件照"+strings.ToLower(path.Ext(*g.photo)))
		ok, err := s.writeZipFile(ctx, zw, dir+"/"+name, *g.photo)
		if err != nil {
			return written, err
		}
		if ok {
			written++
		}
	}
	multi := len(g.audios) > 1
	for _, a := range g.audios {
		base := strings.TrimSpace(a.program)
		if base == "" {
			base = "舞蹈音频"
		}
		if multi {
			base = fmt.Sprintf("%s_%s_%s", base, a.level, danceTypeLabel(a.danceType))
		}
		name := uniqueZipName(used, safeZipName(base)+strings.ToLower(path.Ext(a.path)))
		ok, err := s.writeZipFile(ctx, zw, dir+"/"+name, a.path)
		if err != nil {
			return written, err
		}
		if ok {
			written++
		}
	}
	return written, nil
}

// writeZipFile 打开 uploads 下的文件并流式写入 zip 条目。
// 返回是否实际写入（文件缺失时跳过并返回 false）。
func (s *MaterialArchiveService) writeZipFile(ctx context.Context, zw *zip.Writer, name, relPath string) (bool, error) {
	f, err := s.files.Open(relPath)
	if errors.Is(err, os.ErrNotExist) {
		logger.WarnEvent(ctx, "material.archive.file_missing", "归档时资料文件不存在，已跳过",
			"path", relPath)
		return false, nil
	}
	if err != nil {
		return false, httpx.ErrInternal(err, "读取资料文件失败")
	}
	defer f.Close()
	entry, err := zw.Create(name)
	if err != nil {
		return false, httpx.ErrInternal(err, "生成归档条目失败")
	}
	if _, err := io.Copy(entry, f); err != nil {
		return false, httpx.ErrInternal(err, "写入归档文件失败")
	}
	return true, nil
}
