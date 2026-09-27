package activitypush

import (
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/communityclient"
)

func TestParseKey(t *testing.T) {
	cases := map[string]parsedKey{
		"patch_resource:12":                   {kind: kindResource, resourceID: 12},
		"patch_resource_edit:12:7:20260926":   {kind: kindEdit, resourceID: 12, actorID: 7, day: "20260926"},
		"patch_resource:0":                    {},
		"patch_resource:x":                    {},
		"patch_resource_edit:12:0:20260926":   {},
		"patch_resource_edit:12:7:2026-09-26": {},
		"patch_resource_edit:12:7":            {},
		"community:post:1":                    {},
		"patch_resource_like:12:7":            {},
	}
	for key, want := range cases {
		got, ok := parseKey(key)
		if ok != (want.kind != 0) || got != want {
			t.Errorf("parseKey(%q) = %+v, %v", key, got, ok)
		}
	}
}

func TestEditKeyUsesTheBeijingDay(t *testing.T) {
	at := time.Date(2026, 9, 25, 15, 59, 59, 0, time.UTC)
	if k := editKey(3, 7, at); k != "patch_resource_edit:3:7:20260925" {
		t.Errorf("before Beijing midnight: %s", k)
	}
	if k := editKey(3, 7, at.Add(time.Second)); k != "patch_resource_edit:3:7:20260926" {
		t.Errorf("at Beijing midnight: %s", k)
	}
}

func TestTextIsPlainAndCut(t *testing.T) {
	if got := singleLine("line\none\x00two"); got != "line one two" {
		t.Errorf("singleLine = %q", got)
	}
	if got := withoutControls("a\x08b\r\nc\td"); got != "abcd" {
		t.Errorf("withoutControls = %q", got)
	}
	long := ""
	for range 201 {
		long += "汉"
	}
	if got := []rune(cutRunes(long, titleRunes)); len(got) != 200 {
		t.Errorf("cut to %d runes", len(got))
	}
	if coverHash("ABC") != "" || coverHash("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") == "" {
		t.Error("coverHash accepts only 64 lowercase hex")
	}
	if got := labelled([]string{"manual", "unknown", ""}, typeLabels); got != "人工翻译补丁、unknown" {
		t.Errorf("labelled = %q", got)
	}
}

func TestDiffersComparesEveryPushedField(t *testing.T) {
	work := int64(9)
	at := time.Date(2026, 9, 26, 4, 0, 0, 123456000, time.UTC)
	hash := "h"
	want := &communityclient.ActivityItem{
		Key: "patch_resource:1", ActorID: 7, Verb: "publish", ObjectKind: "patch_resource", ObjectLabel: "Galgame 补丁",
		Title: "t", Excerpt: "e", URL: "u", CoverImageHash: "h", WorkID: &work, ContentLimit: "sfw", OccurredAt: &at,
		Notify: true, Revision: 99,
	}
	have := communityclient.SiteActivity{
		Key: "patch_resource:1", ActorID: 7, Verb: "publish", ObjectKind: "patch_resource", ObjectLabel: "Galgame 补丁",
		Title: "t", Excerpt: "e", URL: "u", CoverImageHash: &hash, WorkID: &work, ContentLimit: "sfw",
		OccurredAt: at.In(beijing), Notify: false, Revision: 5,
	}
	if differs(want, have) {
		t.Fatal("notify, revision or a time zone counted as drift")
	}
	for name, mutate := range map[string]func(*communityclient.SiteActivity){
		"content_limit": func(h *communityclient.SiteActivity) { h.ContentLimit = "nsfw" },
		"title":         func(h *communityclient.SiteActivity) { h.Title = "x" },
		"cover":         func(h *communityclient.SiteActivity) { h.CoverImageHash = nil },
		"work":          func(h *communityclient.SiteActivity) { h.WorkID = nil },
		"actor":         func(h *communityclient.SiteActivity) { h.ActorID = 8 },
		"occurred":      func(h *communityclient.SiteActivity) { h.OccurredAt = at.Add(time.Microsecond) },
	} {
		h := have
		mutate(&h)
		if !differs(want, h) {
			t.Errorf("%s drift not seen", name)
		}
	}
}

func TestOriginIsHTTPSOnly(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.moyu.moe/auth/callback":  "https://www.moyu.moe",
		"http://127.0.0.1:6969/auth/callback": "",
		"":                                    "",
		"https:///nohost":                     "",
	} {
		if got := Origin(in); got != want {
			t.Errorf("Origin(%q) = %q", in, got)
		}
	}
}
