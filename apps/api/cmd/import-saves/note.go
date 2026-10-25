package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

type saveRecord struct {
	AssetID        int64  `json:"asset_id"`
	WorkID         int    `json:"work_id"`
	Path           string `json:"path"`
	Bytes          int64  `json:"bytes"`
	Filename       string `json:"filename"`
	Label          string `json:"label"`
	Site           string `json:"site"`
	SourceURL      string `json:"source_url"`
	GameTitle      string `json:"game_title"`
	OriginalSHA256 string `json:"original_sha256"`
	ConvertedFrom  string `json:"converted_from"`
}

var saveKinds = []struct{ marker, kind string }{
	{"フルコンプ", "全通关存档"},
	{"回想全解放", "回想全开存档"},
	{"回想全開放", "回想全开存档"},
	{"全解放", "全开存档"},
	{"攻略済み", "通关存档"},
}

func saveKind(label string) string {
	for _, k := range saveKinds {
		if strings.Contains(label, k.marker) {
			return k.kind
		}
	}
	return "存档"
}

var sizeOrLinkLabel = regexp.MustCompile(`^(?i:dl|[0-9.,]+\s*[kmg]?b)$`)

// sagaoz labels most downloads with their size and seiya with "DL"; neither
// says anything about the save.
func meaningfulLabel(label string) string {
	l := strings.TrimSpace(norm.NFKC.String(label))
	if sizeOrLinkLabel.MatchString(l) {
		return ""
	}
	return l
}

func resourceName(r saveRecord) string {
	name := strings.TrimSpace(r.GameTitle) + " - " + saveKind(r.Label)
	if rs := []rune(name); len(rs) > 300 {
		name = string(rs[:300])
	}
	return name
}

func resourceNote(r saveRecord, userID int) string {
	host := r.Site
	if u, err := url.Parse(r.SourceURL); err == nil && u.Host != "" {
		host = u.Host
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s，来自 [%s](%s)", saveKind(r.Label), host, r.SourceURL)
	if l := meaningfulLabel(r.Label); l != "" {
		fmt.Fprintf(&b, "，原站说明：%s", l)
	}
	b.WriteString("。\n\n")
	if r.ConvertedFrom == "lzh" {
		b.WriteString("原文件是 LZH 压缩包，已原样转为 zip，文件名改为 UTF-8 编码。\n\n")
	}
	fmt.Fprintf(&b, "原始文件 SHA-256：`%s`\n\n", r.OriginalSHA256)
	fmt.Fprintf(&b, "**本存档由 [LetMoe](https://www.moyu.moe/user/%d/resource) 归档**\n", userID)
	return b.String()
}
