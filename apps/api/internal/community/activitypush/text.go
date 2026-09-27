package activitypush

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	titleRunes   = 200
	excerptRunes = 300
)

var imageHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Mirrors apps/web/app/constants/resource.ts. An unknown value is shown as is.
var (
	typeLabels = map[string]string{
		"manual": "人工翻译补丁", "ai": "AI 翻译补丁", "machine_polishing": "机翻润色",
		"machine": "机翻补丁", "save": "全 CG 存档", "crack": "破解补丁", "fix": "修正补丁",
		"mod": "魔改补丁", "r18": "R18 成人内容补丁", "decensor": "去马赛克补丁",
		"image": "修图补丁", "other": "其它",
	}
	languageLabels = map[string]string{
		"zh-Hans": "简体中文", "zh-Hant": "繁體中文", "ja": "日本語", "en": "English", "other": "其它",
	}
	platformLabels = map[string]string{
		"windows": "Windows", "android": "Android", "macos": "MacOS", "ios": "iOS",
		"linux": "Linux", "other": "其它",
	}
)

// singleLine and withoutControls exist because community refuses a control
// character anywhere in a title or excerpt, and the forum's first backfill lost
// 3,427 items to it.
func singleLine(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func withoutControls(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func cutRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n]))
}

func labelled(values []string, labels map[string]string) string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if l, ok := labels[v]; ok {
			v = l
		}
		out = append(out, v)
	}
	return strings.Join(out, "、")
}

func joinNonEmpty(sep string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func coverHash(h string) string {
	if imageHash.MatchString(h) {
		return h
	}
	return ""
}
