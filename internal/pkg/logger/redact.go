package logger

import (
	"regexp"
	"strings"

	"github.com/meetmonth/dance-elite/internal/utils"
)

// idNumberPattern 匹配 18 位居民身份证号（末位可为 X），用于在自由文本
// （错误信息、回调报文）进入日志前打码。
var idNumberPattern = regexp.MustCompile(`[0-9]{17}[0-9Xx]`)

// phonePattern 匹配大陆手机号的候选串；是否独立成号由前后字符是否为数字判定，
// 避免把订单号（22 位数字）等长数字串中的片段误当手机号。
var phonePattern = regexp.MustCompile(`1[3-9][0-9]{9}`)

// RedactText 把文本中的身份证号、手机号替换为掩码后返回，
// 用于访问日志、错误日志等可能夹带用户输入的字段。日志中绝不出现证件号明文。
// 只对"独立"编号打码：前后若仍是数字，说明它是更长数字串（订单号/流水号）的一部分。
func RedactText(s string) string {
	s = redactStandalone(s, idNumberPattern, utils.MaskIDNumber)
	return redactStandalone(s, phonePattern, utils.MaskPhone)
}

// RedactIDNumber 对单个证件号字段打码；空值原样返回，便于在结构化日志中使用。
func RedactIDNumber(s string) string {
	if s == "" {
		return ""
	}
	return utils.MaskIDNumber(s)
}

// RedactPhone 对单个手机号字段打码；空值原样返回。
func RedactPhone(s string) string {
	if s == "" {
		return ""
	}
	return utils.MaskPhone(s)
}

// redactStandalone 只替换前后都不是数字的匹配串（订单号等长数字串不误伤）。
func redactStandalone(s string, re *regexp.Regexp, mask func(string) string) string {
	locs := re.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if (start > 0 && isDigit(s[start-1])) || (end < len(s) && isDigit(s[end])) {
			continue
		}
		b.WriteString(s[last:start])
		b.WriteString(mask(s[start:end]))
		last = end
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
