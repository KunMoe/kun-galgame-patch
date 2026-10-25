package archive

import (
	"encoding/json"
	"os"
	"testing"
)

func TestDecodeSaveReadsRealMVAndMZSaves(t *testing.T) {
	for _, name := range []string{"testdata/file1.rpgsave", "testdata/file1.rmmzsave"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		out, err := decodeSave(name, data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var save map[string]any
		if err := json.Unmarshal(out, &save); err != nil {
			t.Fatalf("%s decoded to invalid JSON: %v", name, err)
		}
		if len(save) == 0 {
			t.Errorf("%s decoded to an empty object", name)
		}
	}
}

func TestDecodeSaveRefusesWhatIsNotASave(t *testing.T) {
	for name, data := range map[string]string{
		"a.rpgsave":  "not base64 lz data at all",
		"a.rmmzsave": "plain text",
		"a.txt":      "{}",
		"b.rpgsave":  "",
		"c.rmmzsave": "",
	} {
		if _, err := decodeSave(name, []byte(data)); err == nil {
			t.Errorf("decodeSave(%q) accepted %q", name, data)
		}
	}
}
