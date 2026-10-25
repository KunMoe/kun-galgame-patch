package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var sanitizeRe = regexp.MustCompile(`[^\p{L}\p{N}_-]`)

func sanitizeFileName(fileName string) string {
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	base = sanitizeRe.ReplaceAllString(base, "")
	r := []rune(base)
	if len(r) > 100 {
		r = r[:100]
	}
	return string(r) + ext
}

func formatSize(bytes int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if bytes >= gb {
		return fmt.Sprintf("%.3fGB", float64(bytes)/float64(gb))
	}
	return fmt.Sprintf("%.3fMB", float64(bytes)/float64(mb))
}

var nonDigitRe = regexp.MustCompile(`[^0-9]`)

func formatDateYYMMDD(s string) string {
	t := nonDigitRe.ReplaceAllString(s, "")
	switch len(t) {
	case 8:
		return t[2:4] + "-" + t[4:6] + "-" + t[6:8]
	case 6:
		return t[2:4] + "-" + t[4:6]
	case 4:
		return t[2:4]
	default:
		return s
	}
}

func buildResourceName(group, gameName, sanitized string) string {
	g := strings.TrimSpace(gameName)
	if g == "" {
		return sanitized
	}
	name := "【" + strings.TrimSpace(group) + "】" + g + " - 人工汉化补丁"
	if r := []rune(name); len(r) > 300 {
		name = string(r[:300])
	}
	return name
}

func renderNote(p *parsedPatch, sanitized string, archiveUserID int) string {
	company := p.Company
	game := p.GameName
	group := p.GroupName
	start := formatDateYYMMDD(p.StartDate)
	publish := formatDateYYMMDD(p.PublishDate)
	return fmt.Sprintf(
		"%s - %s 中文化补丁\n\n"+
			"由 %s 开坑于 %s, 完成于 %s\n\n"+
			"**本补丁由 [VN视觉小说汉化补丁遗产归档](https://www.moyu.moe/user/%d/resource) 归档**\n\n"+
			"%s\n",
		company, game, group, start, publish, archiveUserID, sanitized,
	)
}
