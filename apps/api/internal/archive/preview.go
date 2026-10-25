package archive

import (
	"path"
	"strings"
)

type Kind struct {
	Key        string   `json:"kind"`
	MaxBytes   int64    `json:"max_bytes"`
	Truncate   bool     `json:"truncate"`
	Extensions []string `json:"extensions"`
}

const textType = "text/plain"

var Kinds = []Kind{
	{
		Key:      "text",
		MaxBytes: 2 << 20,
		Truncate: true,
		Extensions: []string{
			"txt", "md", "log", "nfo", "ini", "cfg", "conf", "inf", "reg",
			"json", "xml", "yaml", "yml", "toml", "csv", "srt", "ass", "ssa",
			"css", "js", "lua", "py", "rb", "rpy", "ks", "tjs", "cs", "ps1",
			"bat", "cmd", "sh", "html", "htm",
		},
	},
	{
		Key:        "image",
		MaxBytes:   20 << 20,
		Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "bmp", "ico", "avif", "svg"},
	},
	{
		Key:        "audio",
		MaxBytes:   30 << 20,
		Extensions: []string{"ogg", "mp3", "wav", "flac", "m4a", "opus", "aac"},
	},
	{
		// Decoded to JSON on the server; MaxBytes bounds the file read, the
		// decoded text is cut at saveTextLimit.
		Key:        "save",
		MaxBytes:   8 << 20,
		Extensions: []string{"rpgsave", "rmmzsave"},
	},
}

var mediaTypes = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"webp": "image/webp",
	"bmp":  "image/bmp",
	"ico":  "image/x-icon",
	"avif": "image/avif",
	"svg":  "image/svg+xml",
	"ogg":  "audio/ogg",
	"mp3":  "audio/mpeg",
	"wav":  "audio/wav",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
	"opus": "audio/ogg",
	"aac":  "audio/aac",
}

var byExtension = func() map[string]Kind {
	out := make(map[string]Kind)
	for _, k := range Kinds {
		for _, ext := range k.Extensions {
			out[ext] = k
		}
	}
	return out
}()

func Lookup(name string) (Kind, string, bool) {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	k, ok := byExtension[ext]
	if !ok {
		return Kind{}, "", false
	}
	if k.Key == "text" || k.Key == "save" {
		return k, textType, true
	}
	return k, mediaTypes[ext], true
}
