// Command lottery-import 从报名 Excel 生成抽签数据 JSON。
//
// 用法：
//
//	go run ./cmd/lottery-import -excel "报名资料.xlsx" -event "优舞盟12月赛事抽签"
//
// 列按表头名称定位，报名表调整列顺序不影响导入；生成的 JSON 写入
// LOTTERY_DIR（默认 data/lottery），文件名即访问 token。重复导入已存在的
// 赛事需要 -reset，避免误清空已抽签结果。
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"github.com/meetmonth/dance-elite/internal/lotteryimport"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/store"
)

const tokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func main() {
	// 可选读取 .env，让 PUBLIC_BASE_URL / LOTTERY_DIR 与后端配置一致；
	// 已存在的环境变量优先，文件不存在不报错。
	_ = godotenv.Load()

	excelPath := flag.String("excel", "", "报名 Excel 文件路径（必填）")
	eventName := flag.String("event", "优舞盟12月赛事抽签", "抽签页标题中的赛事名称")
	outDir := flag.String("out", defaultDir(), "抽签 JSON 输出目录")
	explicitToken := flag.String("token", "", "访问 token（8-64 位字母数字）；留空时自动生成或复用目录中唯一文件")
	reset := flag.Bool("reset", false, "覆盖已存在的抽签数据（会清空已有抽签结果）")
	flag.Parse()

	if strings.TrimSpace(*excelPath) == "" {
		fail("请用 -excel 指定报名 Excel 文件")
	}

	data, warnings, err := lotteryimport.Build(*excelPath, *eventName)
	if err != nil {
		fail(err.Error())
	}

	token, err := resolveToken(*outDir, *explicitToken)
	if err != nil {
		fail(err.Error())
	}
	lotteryStore := store.NewLotteryStore(*outDir)
	if lotteryStore.Exists(token) && !*reset {
		fail(fmt.Sprintf("抽签文件已存在：%s\n如需用当前 Excel 覆盖（会清空已有抽签结果），请加 -reset",
			filepath.Join(*outDir, token+".json")))
	}
	if err := lotteryStore.Save(context.Background(), token, data); err != nil {
		fail("写入抽签文件失败: " + err.Error())
	}

	fmt.Printf("校验通过：%d 名选手\n", len(data.Players))
	fmt.Println("分组：")
	for _, line := range groupLines(data) {
		fmt.Println("  " + line)
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "提示："+warning)
	}
	fmt.Printf("已写入：%s\n", filepath.Join(*outDir, token+".json"))
	fmt.Printf("页面地址：%s\n", baseURL()+"/lottery/"+token)
}

// resolveToken 确定要写入的 token：显式指定优先；否则复用目录中唯一的
// 抽签文件（便于 -reset 覆盖同一个链接）；目录为空时生成新 token。
func resolveToken(dir, explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		if !store.ValidLotteryToken(explicit) {
			return "", fmt.Errorf("token 不合法：需为 8-64 位字母或数字")
		}
		return explicit, nil
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return "", fmt.Errorf("扫描输出目录失败: %w", err)
	}
	tokens := make([]string, 0, len(files))
	for _, file := range files {
		token := strings.TrimSuffix(filepath.Base(file), ".json")
		if store.ValidLotteryToken(token) {
			tokens = append(tokens, token)
		}
	}
	switch len(tokens) {
	case 0:
		return newToken()
	case 1:
		return tokens[0], nil
	default:
		sort.Strings(tokens)
		return "", fmt.Errorf("输出目录已有 %d 个抽签文件（%s），请用 -token 指定要覆盖哪一个",
			len(tokens), strings.Join(tokens, "、"))
	}
}

// groupLines 输出每个「组别 × 舞种」的人数与字母组划分，便于人工核对。
func groupLines(data *model.LotteryData) []string {
	labels := make(map[string]string, len(data.Genres))
	for _, genre := range data.Genres {
		labels[genre.Key] = genre.Label
	}
	type scope struct{ genre, ageGroup string }
	var order []scope
	sizes := make(map[scope][]string)
	total := make(map[scope]int)
	for _, group := range data.Groups {
		key := scope{group.Genre, group.AgeGroup}
		if _, ok := sizes[key]; !ok {
			order = append(order, key)
		}
		sizes[key] = append(sizes[key], fmt.Sprintf("%s%d", group.Letter, group.Size))
		total[key] += group.Size
	}
	lines := make([]string, 0, len(order))
	for _, key := range order {
		label := labels[key.genre]
		if label == "" {
			label = key.genre
		}
		lines = append(lines, fmt.Sprintf("%s %s：%d 人 → %s",
			key.ageGroup, label, total[key], strings.Join(sizes[key], " ")))
	}
	return lines
}

// baseURL 读取对外访问地址；未配置时返回空串，输出相对路径。
func baseURL() string {
	return strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")
}

// defaultDir 与后端配置保持一致：LOTTERY_DIR，默认 data/lottery。
func defaultDir() string {
	if dir := os.Getenv("LOTTERY_DIR"); dir != "" {
		return dir
	}
	return "data/lottery"
}

// newToken 生成 16 位 crypto/rand 字母数字 token。
func newToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成 token 失败: %w", err)
	}
	for i := range buf {
		buf[i] = tokenAlphabet[int(buf[i])%len(tokenAlphabet)]
	}
	return string(buf), nil
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
