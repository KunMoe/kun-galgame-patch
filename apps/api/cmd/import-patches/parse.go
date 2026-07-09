package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

// parsedPatch is the structured form of a standardized patch filename, ported
// from the legacy sync-patch tool's vn-sync/parse.ts. The naming convention is
// unchanged from the old moyu:
//
//	[会社][YYYYMMDD]游戏名[v31700][Windows][汉化组][YYYYMMDD][CHS].rar
//	  ↑company ↑startDate ↑title ↑vndbId  ↑platform ↑group  ↑publishDate ↑lang
type parsedPatch struct {
	Company     string
	StartDate   string
	GameName    string
	VndbID      string // with leading 'v', e.g. "v31700"
	PlatformRaw string
	Platform    string // "windows" | "other"
	GroupName   string
	PublishDate string
	LangRaw     string   // "CHS" | "CHT" | "CHS&CHT"
	Languages   []string // e.g. ["zh-Hans"] or ["zh-Hans","zh-Hant"] for CHS&CHT
	FileName    string
	FilePath    string
}

var (
	bracketRe       = regexp.MustCompile(`\[[^\]]+\]`)
	bracketInnerRe  = regexp.MustCompile(`\[([^\]]+)\]`)
	vndbRe          = regexp.MustCompile(`(?i)v(\d{1,6})`)
	windowsKeywords = []string{"windows", "win32", "win64", "win"}
	extStripRe      = regexp.MustCompile(`\.[^.]+$`)
)

// normalizeLanguages maps the language bracket to the resource's language list.
// Real archive names carry combined values like "CHS&CHT" (simplified +
// traditional in one release) — patch_resource.language is an array, so both are
// kept. Defaults to zh-Hans when nothing recognizable is present.
func normalizeLanguages(lang string) []string {
	s := strings.ToUpper(lang)
	var out []string
	if strings.Contains(s, "CHS") {
		out = append(out, "zh-Hans")
	}
	if strings.Contains(s, "CHT") {
		out = append(out, "zh-Hant")
	}
	if len(out) == 0 {
		return []string{"zh-Hans"}
	}
	return out
}

func normalizePlatform(p string) string {
	s := strings.ToLower(p)
	for _, k := range windowsKeywords {
		if strings.Contains(s, k) {
			return "windows"
		}
	}
	return "other"
}

// parsePatchFileName parses a standardized patch filename. Returns nil when the
// name does not match the convention (no parseable [v####] VNDB segment) — the
// caller records it as unrecognized and skips, keeping the batch robust.
func parsePatchFileName(filePath string) *parsedPatch {
	fileName := filepath.Base(filePath)
	withoutExt := extStripRe.ReplaceAllString(fileName, "")

	// Positions of every [..] group, so the title = text between the 2nd and 3rd.
	locs := bracketRe.FindAllStringIndex(withoutExt, -1)
	inner := bracketInnerRe.FindAllStringSubmatch(withoutExt, -1)
	if len(locs) < 7 || len(inner) < 7 {
		return nil
	}

	title := strings.TrimSpace(withoutExt[locs[1][1]:locs[2][0]])

	get := func(i int) string {
		if i < len(inner) {
			return strings.TrimSpace(inner[i][1])
		}
		return ""
	}
	company, startDate := get(0), get(1)
	vPart, platformRaw := get(2), get(3)
	groupName, publishDate, langRaw := get(4), get(5), get(6)

	vm := vndbRe.FindStringSubmatch(vPart)
	if vm == nil {
		return nil
	}
	vndbID := "v" + vm[1]

	if langRaw == "" {
		langRaw = "CHS"
	}
	return &parsedPatch{
		Company:     company,
		StartDate:   startDate,
		GameName:    title,
		VndbID:      vndbID,
		PlatformRaw: platformRaw,
		Platform:    normalizePlatform(platformRaw),
		GroupName:   groupName,
		PublishDate: publishDate,
		LangRaw:     langRaw,
		Languages:   normalizeLanguages(langRaw),
		FileName:    fileName,
		FilePath:    filePath,
	}
}
