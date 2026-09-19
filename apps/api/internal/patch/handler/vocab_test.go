package handler

import (
	"testing"

	face "kun-galgame-patch-api/internal/face/service"
	"kun-galgame-patch-api/internal/patch/dto"
)

func TestValidateResourceVocab(t *testing.T) {
	all := dto.PatchResourceCreateRequest{Type: face.PatchTypes, Language: face.PatchLanguages, Platform: face.PatchPlatforms}
	if msg := validateResourceVocab(&all); msg != "" {
		t.Fatalf("the whole vocabulary was refused: %s", msg)
	}

	zh, win := []string{"zh-Hans"}, []string{"windows"}
	refused := map[string]dto.PatchResourceCreateRequest{
		"unknown type":       {Type: []string{"manual", "voice"}, Language: zh, Platform: win},
		"unknown language":   {Type: []string{"manual"}, Language: []string{"jp"}, Platform: win},
		"unknown platform":   {Type: []string{"manual"}, Language: zh, Platform: []string{"Windows"}},
		"label, not a value": {Type: []string{"人工翻译"}, Language: zh, Platform: win},
	}
	for name, req := range refused {
		if msg := validateResourceVocab(&req); msg == "" {
			t.Errorf("%s: accepted, want refused", name)
		}
	}
}
