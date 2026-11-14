// Package web 内嵌后台管理端与音频审核 H5 的前端构建产物。
//
// 产物由 dance-admin 的 `pnpm build` 从 dance-admin/dist 同步到本包的 dist/，
// 目录内容不入库（仅保留 .gitkeep）。因此发布构建前必须先构建前端，
// 否则二进制内没有页面资源，路由层会跳过托管。
package web

import (
	"embed"
	"io/fs"
)

// 必须用 all: 前缀：Umi 产物大量以 "_" 开头的文件（如 _0dl27-xxx.css），
// 普通 dist 模式会跳过它们，导致页面白屏。
//
//go:embed all:dist
var embedded embed.FS

// Dist 返回前端构建产物根目录（dist/ 内部）。
// 前端未构建时返回的 FS 只含 .gitkeep，调用方可据此判断是否启用页面托管。
func Dist() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
