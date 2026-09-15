package handler

import "testing"

func TestValidateBotResource(t *testing.T) {
	zh := []string{"zh-Hans"}
	win := []string{"windows"}

	refused := []struct {
		name  string
		types []string
		langs []string
		plats []string
	}{
		{"crack", []string{"crack"}, zh, win},
		{"decensor", []string{"decensor"}, zh, win},
		{"r18", []string{"r18"}, zh, win},
		{"mod", []string{"mod"}, zh, win},
		{"save", []string{"save"}, zh, win},
		{"image", []string{"image"}, zh, win},
		{"other", []string{"other"}, zh, win},
		{"unknown type", []string{"nope"}, zh, win},
		{"one bad type among good", []string{"manual", "crack"}, zh, win},
		{"unknown language", []string{"manual"}, []string{"jp"}, win},
		{"unknown platform", []string{"manual"}, zh, []string{"zh-Hans"}},
	}
	for _, c := range refused {
		if msg := validateBotResource(c.types, c.langs, c.plats); msg == "" {
			t.Errorf("%s: accepted, want refused", c.name)
		}
	}

	accepted := []struct {
		name  string
		types []string
		langs []string
		plats []string
	}{
		{"manual zh-Hans", []string{"manual"}, zh, win},
		{"fix ja", []string{"fix"}, []string{"ja"}, win},
		{"ai + machine_polishing", []string{"ai", "machine_polishing"}, zh, win},
		{"machine android", []string{"machine"}, zh, []string{"android"}},
	}
	for _, c := range accepted {
		if msg := validateBotResource(c.types, c.langs, c.plats); msg != "" {
			t.Errorf("%s: %s", c.name, msg)
		}
	}
}
