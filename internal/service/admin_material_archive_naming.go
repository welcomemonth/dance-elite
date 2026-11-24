package service

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
)

// normalizeArchivePlayers 校验并规范化 player_nos 过滤参数（去重、保留顺序）。
func normalizeArchivePlayers(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			no := strings.TrimSpace(part)
			if no == "" || seen[no] {
				continue
			}
			if !playerNoPattern.MatchString(no) {
				return nil, httpx.ErrBadRequest("player_nos 含非法选手编号")
			}
			seen[no] = true
			out = append(out, no)
		}
	}
	if len(out) > materialArchiveMaxPlayers {
		return nil, httpx.ErrBadRequest(fmt.Sprintf("一次最多导出 %d 名选手", materialArchiveMaxPlayers))
	}
	return out, nil
}

// archiveUnsafePattern 匹配 zip 名称中的路径分隔符与控制字符。
var archiveUnsafePattern = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)

// playerNoPattern 是系统生成的选手编号格式（8 位字母数字）。
var playerNoPattern = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)

// safeZipName 清理 zip 目录/文件名中的非法字符，避免路径穿越与解压异常。
func safeZipName(s string) string {
	s = archiveUnsafePattern.ReplaceAllString(strings.TrimSpace(s), "_")
	s = strings.Trim(s, ". ")
	if s == "" {
		return "未命名"
	}
	return s
}

// uniqueZipName 处理同一目录内的重名（追加 _2、_3）。
func uniqueZipName(used map[string]bool, name string) string {
	if !used[name] {
		used[name] = true
		return name
	}
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

// archiveRootName 生成 zip 根目录名：赛事名_赛事ID。
func archiveRootName(eventName string, eventID int64) string {
	return safeZipName(eventName) + "_" + strconv.FormatInt(eventID, 10)
}
