package main

import (
	"reflect"
	"testing"
)

// Cases use REAL filenames from the archive's Filelist.txt (增量6) to lock in the
// parser against the naming actually shipped: combined CHS&CHT, non-Windows
// platforms, and titles with hyphens / punctuation / fullwidth chars.
func TestParsePatchFileName(t *testing.T) {
	cases := []struct {
		name      string
		file      string
		wantNil   bool
		vndb      string
		platform  string
		languages []string
		title     string
		group     string
	}{
		{
			name:      "windows CHS single",
			file:      "[0verflow（オーバーフロー）][20251219]School Days REMASTERED[v14][Windows][NO DATA][20260517][CHS].rar",
			vndb:      "v14",
			platform:  "windows",
			languages: []string{"zh-Hans"},
			title:     "School Days REMASTERED",
			group:     "NO DATA",
		},
		{
			name:      "windows CHS&CHT dual language",
			file:      "[CloverGAME][20251031]やりなおしクランクイン[v59027][Windows][Can／need&V1.0][20260509][CHS&CHT].rar",
			vndb:      "v59027",
			platform:  "windows",
			languages: []string{"zh-Hans", "zh-Hant"},
			title:     "やりなおしクランクイン",
			group:     "Can／need&V1.0",
		},
		{
			name:      "nintendo switch -> other, title with hyphens/apostrophe",
			file:      "[FAVORITE][20230222]さくら、もゆ。-as the Night's, Reincarnation-[v22313][Nintendo Switch][萌譯×F廚の米線×ricecake個人製作][20260222][CHS].rar",
			vndb:      "v22313",
			platform:  "other",
			languages: []string{"zh-Hans"},
			title:     "さくら、もゆ。-as the Night's, Reincarnation-",
			group:     "萌譯×F廚の米線×ricecake個人製作",
		},
		{
			name:      "playstation 2 -> other",
			file:      "[GameCRAB（ゲームクラブ）][20010405]トゥルーラブストーリー3[v4468][PlayStation 2][偷樂小神仙][20260624][CHS].rar",
			vndb:      "v4468",
			platform:  "other",
			languages: []string{"zh-Hans"},
			title:     "トゥルーラブストーリー3",
		},
		{
			name:      "playstation vita -> other, title with hyphens",
			file:      "[フリュー株式会社][20151105]To LOVEる-とらぶる- ダークネス トゥループリンセス[v18027][PlayStation Vita][PSV汉化計劃][20260101][CHS].rar",
			vndb:      "v18027",
			platform:  "other",
			languages: []string{"zh-Hans"},
			title:     "To LOVEる-とらぶる- ダークネス トゥループリンセス",
		},
		{
			name:    "unrecognized filename",
			file:    "random_backup_file.rar",
			wantNil: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parsePatchFileName(c.file)
			if c.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected a parse, got nil")
			}
			if got.VndbID != c.vndb {
				t.Errorf("VndbID = %q, want %q", got.VndbID, c.vndb)
			}
			if got.Platform != c.platform {
				t.Errorf("Platform = %q, want %q", got.Platform, c.platform)
			}
			if !reflect.DeepEqual(got.Languages, c.languages) {
				t.Errorf("Languages = %v, want %v", got.Languages, c.languages)
			}
			if got.GameName != c.title {
				t.Errorf("GameName = %q, want %q", got.GameName, c.title)
			}
			if c.group != "" && got.GroupName != c.group {
				t.Errorf("GroupName = %q, want %q", got.GroupName, c.group)
			}
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	// Extension preserved; separators/brackets/punctuation stripped from the base;
	// CJK kept (\p{L}). This is the (galgame_id, name) dedup key.
	in := "[Key][20000908]AIR[v36][Windows][Key Fans Club][20050104][CHS].rar"
	got := sanitizeFileName(in)
	if got == "" || got[len(got)-4:] != ".rar" {
		t.Fatalf("expected a .rar name, got %q", got)
	}
	for _, bad := range []string{"[", "]", " ", "&", "／"} {
		if containsStr(got, bad) {
			t.Errorf("sanitized name %q still contains %q", got, bad)
		}
	}
}

func containsStr(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
