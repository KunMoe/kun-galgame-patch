package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var sanitizeRe = regexp.MustCompile(`[^\p{L}\p{N}_-]`)

// sanitizeFileName mirrors the legacy lib/sanitizeFileName.ts: keep letters,
// digits, underscore and hyphen in the base name (CJK included via \p{L}),
// truncate to 100 runes, preserve the extension. This is BOTH the artifact
// download filename and this importer's per-galgame dedup key (resource.Name).
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

// formatSize mirrors formatSizeString: GB with 3 decimals at/above 1 GiB, else MB.
func formatSize(bytes int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if bytes >= gb {
		return fmt.Sprintf("%.3fGB", float64(bytes)/float64(gb))
	}
	return fmt.Sprintf("%.3fMB", float64(bytes)/float64(mb))
}

var nonDigitRe = regexp.MustCompile(`[^0-9]`)

// formatDateYYMMDD mirrors formatDateYYMMDD: 8 digits -> YY-MM-DD, 6 -> YY-MM,
// 4 -> YY, otherwise the original string.
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

// mimeForExt maps the archive extension to a MIME type for the artifact init
// call (the site's artifact_allowed_mime may match on MIME or extension).
func mimeForExt(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".zip":
		return "application/zip"
	case ".7z":
		return "application/x-7z-compressed"
	case ".rar":
		return "application/x-rar-compressed"
	default:
		return "application/octet-stream"
	}
}

// buildResourceName builds the resource display name in the site-wide 2310
// convention: 【<汉化组名>】<游戏名> - 人工汉化补丁. Falls back to the sanitized
// filename when no game title was parsed (a few archive files carry an empty
// title). patch_resource.name is varchar(300); truncate defensively (real names
// top out ~124 runes). Dedup no longer relies on name == sanitized — it matches
// the sanitized filename inside `note` (strpos), so this format is safe.
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

// renderNote builds the resource note from the legacy vn-sync/note.md template,
// with the archive account id parameterized (was 9147, now the importer's
// --user-id). Kept inline (single small template) rather than reading a file so
// the binary is self-contained when scp'd to prod.
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
