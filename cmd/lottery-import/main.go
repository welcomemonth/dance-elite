// Command lottery-import 从报名 Excel 生成抽签数据 JSON。
//
// 用法：
//
//	go run ./cmd/lottery-import -excel "报名资料.xlsx" -event "优舞盟12月赛事抽签"
//
// 列按表头名称定位，报名表调整列顺序不影响导入；生成的 JSON 写入
// -out 指定的目录（默认 data/lottery，需与 config.yaml 的 storage.lottery_dir
// 一致），文件名即访问 token。重复导入已存在的赛事需要 -reset，避免误清空
// 已抽签结果。日常导入与导出已可在后台「赛事抽签」页完成，本命令保留给
// 无后台访问权限的运维场景和批量脚本使用。
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
	"time"

	"github.com/joho/godotenv"
	"github.com/meetmonth/dance-elite/internal/lotteryimport"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

const tokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func main() {
	_ = godotenv.Load()

	excelPath := flag.String("excel", "", "报名 Excel 文件路径（必填）")
	eventName := flag.String("event", "优舞盟12月赛事抽签", "抽签页标题中的赛事名称")
	outDir := flag.String("out", defaultDir(), "抽签 JSON 输出目录")
	explicitToken := flag.String("token", "", "访问 token（8-64 位字母数字）；留空时自动生成或复用目录中唯一文件")
	reset := flag.Bool("reset", false, "覆盖已存在的抽签数据（会清空已有抽签结果与短信发送记录）")
	startAt := flag.String("start-at", "", "抽签开始时间，如 \"2026-11-25 10:00\" 或 ISO 8601；留空不限制")
	endAt := flag.String("end-at", "", "抽签截止时间；到点后未抽选手由系统自动分配；留空不限制")
	baseURL := flag.String("base-url", "", "对外访问地址，用于打印抽签链接与短信模板；留空时打印相对路径")
	remindTemplate := flag.String("remind-template", "", "阿里云提醒短信模板 CODE；留空表示不发提醒短信")
	resultTemplate := flag.String("result-template", "", "阿里云结果短信模板 CODE；留空表示不发结果短信")
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
	exists := lotteryStore.Exists(token)
	if exists && !*reset {
		fail(fmt.Sprintf("抽签文件已存在：%s\n如需用当前 Excel 覆盖（会清空已有抽签结果），请加 -reset",
			filepath.Join(*outDir, token+".json")))
	}

	startValue, endValue, err := resolveWindow(*startAt, *endAt)
	if err != nil {
		fail(err.Error())
	}
	data.StartAt, data.EndAt = startValue, endValue
	if value := strings.TrimSpace(*remindTemplate); value != "" {
		data.SMSRemindTemplate = value
	}
	if value := strings.TrimSpace(*resultTemplate); value != "" {
		data.SMSResultTemplate = value
	}
	if exists {
		// -reset 只清空抽签结果与短信记录；时间窗口与模板 code 未显式给出时沿用旧文件，
		// 避免重新导入把手工改好的配置冲掉。
		previous, err := lotteryStore.Load(context.Background(), token)
		if err != nil {
			fail("读取已有抽签文件失败: " + err.Error())
		}
		preserveSchedule(data, previous)
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
	fmt.Printf("页面地址：%s\n", strings.TrimRight(*baseURL, "/")+"/lottery/"+token)
	printSchedule(data)
	printSMSTemplates(data, token, *baseURL)
}

// resolveWindow 解析 -start-at / -end-at 并校验先后顺序，统一转成 ISO 本地时间。
func resolveWindow(startRaw, endRaw string) (string, string, error) {
	var start, end time.Time
	var hasStart, hasEnd bool
	if strings.TrimSpace(startRaw) != "" {
		t, err := utils.ParseFlexibleTime(startRaw)
		if err != nil {
			return "", "", fmt.Errorf("-start-at: %w", err)
		}
		start, hasStart = t, true
	}
	if strings.TrimSpace(endRaw) != "" {
		t, err := utils.ParseFlexibleTime(endRaw)
		if err != nil {
			return "", "", fmt.Errorf("-end-at: %w", err)
		}
		end, hasEnd = t, true
	}
	if hasStart && hasEnd && !start.Before(end) {
		return "", "", fmt.Errorf("-start-at 必须早于 -end-at")
	}
	startValue, endValue := "", ""
	if hasStart {
		startValue = utils.FormatISO(start)
	}
	if hasEnd {
		endValue = utils.FormatISO(end)
	}
	return startValue, endValue, nil
}

// preserveSchedule 在重新导入时沿用旧文件的抽签时间窗口与短信模板配置。
func preserveSchedule(data, previous *model.LotteryData) {
	if data.StartAt == "" {
		data.StartAt = previous.StartAt
	}
	if data.EndAt == "" {
		data.EndAt = previous.EndAt
	}
	if data.RemindBeforeMinutes == 0 {
		data.RemindBeforeMinutes = previous.RemindBeforeMinutes
	}
	if data.SMSRemindTemplate == "" {
		data.SMSRemindTemplate = previous.SMSRemindTemplate
	}
	if data.SMSResultTemplate == "" {
		data.SMSResultTemplate = previous.SMSResultTemplate
	}
}

// printSchedule 输出抽签时间窗口，便于人工核对。
func printSchedule(data *model.LotteryData) {
	if data.StartAt == "" && data.EndAt == "" {
		fmt.Println("抽签时间：未配置（随时可抽，不自动分配、不发短信）")
		return
	}
	if data.StartAt != "" {
		fmt.Println("开始时间：" + data.StartAt)
	}
	if data.EndAt != "" {
		fmt.Println("截止时间：" + data.EndAt)
	}
}

// printSMSTemplates 输出当前模板配置；有缺失时打印可直接提交阿里云审核的文案。
// 阿里云不允许把链接放进变量，链接必须写死在模板正文，因此每个赛事单独提审。
func printSMSTemplates(data *model.LotteryData, token, baseURL string) {
	if data.EndAt == "" {
		return
	}
	fmt.Println()
	fmt.Printf("短信模板：提醒 %s，结果 %s\n",
		templateOrUnset(data.SMSRemindTemplate), templateOrUnset(data.SMSResultTemplate))

	missingRemind := data.SMSRemindTemplate == ""
	missingResult := data.SMSResultTemplate == ""
	if !missingRemind && !missingResult {
		return
	}
	link := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if link == "" {
		link = "http://<请先配置 PUBLIC_BASE_URL>"
	}
	link += "/lottery/" + token
	fmt.Println("以下模板需在阿里云短信控制台提交审核（链接写死在正文，每赛事单独提审），")
	fmt.Println("通过后用 -remind-template / -result-template 重新导入，或在后台抽签页的赛事设置中填写：")
	if missingRemind {
		fmt.Println("  [提醒] 尊敬的${name}家长您好，${event}的抽签将于${deadline}截止，请点击链接完成抽签：" + link)
		fmt.Println("         变量：name/event/deadline")
	}
	if missingResult {
		fmt.Println("  [结果] 尊敬的${name}家长您好，${event}抽签已截止，您的出场顺序为${slot}（${genre}），点击链接查询详情：" + link)
		fmt.Println("         变量：name/event/slot/genre")
	}
	fmt.Println("  建议 name 按「其他」类型报备（同手机号多个选手会合并为「张三、李四」）。")
}

// templateOrUnset 把空模板 code 显示为「未配置」，避免输出里出现空串。
func templateOrUnset(code string) string {
	if code == "" {
		return "未配置（不发）"
	}
	return code
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

// defaultDir 与 config.yaml 的 storage.lottery_dir 默认值保持一致。
func defaultDir() string {
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
