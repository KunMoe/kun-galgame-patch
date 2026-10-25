package archive

import "testing"

func TestEveryMediaExtensionHasAType(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Kinds {
		for _, ext := range k.Extensions {
			if seen[ext] {
				t.Errorf("%s is listed twice", ext)
			}
			seen[ext] = true
			if k.Key != "text" && k.Key != "save" && mediaTypes[ext] == "" {
				t.Errorf("%s (%s) has no media type", ext, k.Key)
			}
		}
	}
	for ext := range mediaTypes {
		if !seen[ext] {
			t.Errorf("media type for %s, which no kind lists", ext)
		}
	}
}

func TestLookup(t *testing.T) {
	for _, tc := range []struct {
		name, kind, mediaType string
	}{
		{"work/お読みください.TXT", "text", "text/plain"},
		{"work/index.html", "text", "text/plain"},
		{"work/cg/001.PNG", "image", "image/png"},
		{"work/icon.svg", "image", "image/svg+xml"},
		{"work/bgm/title.ogg", "audio", "audio/ogg"},
		{"www/save/file1.rpgsave", "save", "text/plain"},
		{"save/file1.rmmzsave", "save", "text/plain"},
	} {
		k, mt, ok := Lookup(tc.name)
		if !ok || k.Key != tc.kind || mt != tc.mediaType {
			t.Errorf("Lookup(%q) = %s %s %v", tc.name, k.Key, mt, ok)
		}
	}
	for _, name := range []string{"work/Game.exe", "work/img/a.png_", "work/a.rpgmvp", "work/README", "work/", "work/movie.mp4"} {
		if _, _, ok := Lookup(name); ok {
			t.Errorf("Lookup(%q) opened a file outside the allowlist", name)
		}
	}
}
