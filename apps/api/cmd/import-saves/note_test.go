package main

import (
	"strings"
	"testing"
)

func TestSaveKindReadsTheSourceLabel(t *testing.T) {
	for label, want := range map[string]string{
		"フルコンプセーブ":      "全通关存档",
		"回想全解放済みセーブデータ": "回想全开存档",
		"回想全開放済みセーブデータ": "回想全开存档",
		"全解放済みセーブデータ":   "全开存档",
		"攻略済みセーブデータ":    "通关存档",
		"セーブデータ":        "存档",
		"１５０８ＫＢ":        "存档",
	} {
		if got := saveKind(label); got != want {
			t.Errorf("saveKind(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestNoteDropsSizeLabelsAndNamesTheSource(t *testing.T) {
	r := saveRecord{Label: "１５０８ＫＢ", Filename: "x.zip", SourceURL: "https://sagaoz.net/savedata/x.html", OriginalSHA256: "abc", ConvertedFrom: "lzh"}
	note := resourceNote(r)
	if strings.Contains(note, "原站说明") {
		t.Errorf("a size label is not a description:\n%s", note)
	}
	for _, want := range []string{"[sagaoz.net](https://sagaoz.net/savedata/x.html)", "LZH", "`abc`", "`x.zip`"} {
		if !strings.Contains(note, want) {
			t.Errorf("note lacks %q:\n%s", want, note)
		}
	}
	if note := resourceNote(saveRecord{Label: "DL", SourceURL: "https://seiya-saiga.com/a"}); strings.Contains(note, "原站说明") {
		t.Errorf("DL is a link label, not a description:\n%s", note)
	}
	if note := resourceNote(saveRecord{Label: "回想全解放済みセーブデータ"}); !strings.Contains(note, "原站说明：回想全解放済みセーブデータ") {
		t.Errorf("a real description is kept:\n%s", note)
	}
}
