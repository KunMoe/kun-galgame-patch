package handler

import (
	"fmt"
	"strings"
)

const (
	fieldWorkDisplayName   = "catalog.work.display_name"
	fieldWorkOLang         = "catalog.work.olang"
	fieldWorkContentRating = "catalog.work.content_rating"
	fieldWorkTitles        = "catalog.work.titles"
	fieldWorkIntros        = "catalog.work.intros"
	fieldWorkDisplayNSFW   = "catalog.work.display_nsfw"
)

const (
	titleKindOfficial = 0
	titleKindAlias    = 1
)

const (
	contentRatingAllAges = 0
	contentRatingR18     = 2
)

var wizardLangs = []struct {
	form     string
	registry string
}{
	{"zh_cn", "zh-Hans"},
	{"zh_tw", "zh-Hant"},
	{"ja_jp", "ja"},
	{"en_us", "en"},
}

func registryLang(form string) (string, bool) {
	norm := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(form)), "-", "_")
	for _, l := range wizardLangs {
		if l.form == norm || strings.EqualFold(l.registry, strings.TrimSpace(form)) {
			return l.registry, true
		}
	}
	return "", false
}

type SubmissionForm struct {
	NameZhCn         string `json:"name_zh_cn"`
	NameZhTw         string `json:"name_zh_tw"`
	NameJaJp         string `json:"name_ja_jp"`
	NameEnUs         string `json:"name_en_us"`
	IntroZhCn        string `json:"intro_zh_cn"`
	Aliases          string `json:"aliases"`
	ContentLimit     string `json:"content_limit"`
	AgeLimit         string `json:"age_limit"`
	OriginalLanguage string `json:"original_language"`
	SubmitKey        string `json:"submit_key"`
}

func (f *SubmissionForm) nameFor(form string) string {
	switch form {
	case "zh_cn":
		return strings.TrimSpace(f.NameZhCn)
	case "zh_tw":
		return strings.TrimSpace(f.NameZhTw)
	case "ja_jp":
		return strings.TrimSpace(f.NameJaJp)
	case "en_us":
		return strings.TrimSpace(f.NameEnUs)
	}
	return ""
}

func (f *SubmissionForm) SubmissionFields() (map[string]any, error) {
	titles := make([]any, 0, len(wizardLangs)+4)
	displayName := ""
	for _, l := range wizardLangs {
		name := f.nameFor(l.form)
		if name == "" {
			continue
		}
		if displayName == "" {
			displayName = name
		}
		titles = append(titles, map[string]any{
			"lang": l.registry, "title": name, "kind": titleKindOfficial,
		})
	}
	if displayName == "" {
		return nil, fmt.Errorf("至少填写一个语言的名称")
	}

	seen := map[string]struct{}{}
	for _, alias := range strings.Split(f.Aliases, ",") {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if _, dup := seen[alias]; dup {
			continue
		}
		seen[alias] = struct{}{}
		titles = append(titles, map[string]any{
			"lang": "", "title": alias, "kind": titleKindAlias,
		})
	}

	olang, ok := registryLang(f.OriginalLanguage)
	if !ok {
		return nil, fmt.Errorf("原始语言不在可选范围内")
	}

	fields := map[string]any{
		fieldWorkDisplayName:   displayName,
		fieldWorkOLang:         olang,
		fieldWorkTitles:        titles,
		fieldWorkContentRating: contentRatingAllAges,
		fieldWorkDisplayNSFW:   strings.EqualFold(f.ContentLimit, "nsfw"),
	}
	if strings.EqualFold(f.AgeLimit, "r18") {
		fields[fieldWorkContentRating] = contentRatingR18
	}

	if intro := strings.TrimSpace(f.IntroZhCn); intro != "" {
		fields[fieldWorkIntros] = []any{
			map[string]any{"lang": "zh-Hans", "intro": intro},
		}
	}
	return fields, nil
}
