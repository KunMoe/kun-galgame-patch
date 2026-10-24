package client

import (
	"reflect"
	"testing"
)

func TestSFWDetailWithholdsRatedCoversAndSexualTags(t *testing.T) {
	w := &catalogWork{
		ID: 7,
		Covers: []catalogDetailCover{
			{URL: "https://img.example/aa.webp", Sexual: ptr(0)},
			{URL: "https://img.example/bb.webp", Sexual: ptr(1)},
			{URL: "https://img.example/cc.webp"},
		},
		Tags: []catalogWorkTag{
			{Name: "content", CanonicalID: 1},
			{Name: "sexual", CanonicalID: 2, Sexual: true},
		},
	}

	sfw := catalogWorkToFull(w, false)
	if len(sfw.Covers) != 1 || sfw.Covers[0].ImageHash != "aa" {
		t.Errorf("sfw covers = %+v, want only the safe one", sfw.Covers)
	}
	if len(sfw.Tag) != 1 || sfw.Tag[0].Tag.Name != "content" {
		t.Errorf("sfw tags = %+v, want only the non-sexual one", sfw.Tag)
	}

	all := catalogWorkToFull(w, true)
	if len(all.Covers) != 3 || len(all.Tag) != 2 {
		t.Errorf("revealed = %d covers, %d tags, want 3 and 2", len(all.Covers), len(all.Tag))
	}
}

func TestWithholdScreenshots(t *testing.T) {
	shots := []ScreenshotInput{
		{ImageHash: "safe", Sexual: ptr(0)},
		{ImageHash: "suggestive", Sexual: ptr(1)},
		{ImageHash: "explicit", Sexual: ptr(2)},
		{ImageHash: "unassessed"},
	}
	hashes := func(in []ScreenshotInput) []string {
		out := []string{}
		for _, s := range in {
			out = append(out, s.ImageHash)
		}
		return out
	}

	cases := []struct {
		name         string
		contentLimit string
		optIn        []int
		kept         []string
		withheld     map[int]int
	}{
		{"sfw", "sfw", nil, []string{"safe"}, map[int]int{1: 2, 2: 1}},
		{"sfw opted into level 1", "sfw", []int{1}, []string{"safe", "suggestive", "unassessed"}, map[int]int{2: 1}},
		{"absent limit reads as sfw", "", nil, []string{"safe"}, map[int]int{1: 2, 2: 1}},
		{"all", "all", nil, []string{"safe", "suggestive", "explicit", "unassessed"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kept, withheld := WithholdScreenshots(shots, tc.contentLimit, tc.optIn)
			if got := hashes(kept); !reflect.DeepEqual(got, tc.kept) {
				t.Errorf("kept = %v, want %v", got, tc.kept)
			}
			if !reflect.DeepEqual(withheld, tc.withheld) {
				t.Errorf("withheld = %v, want %v", withheld, tc.withheld)
			}
		})
	}
}
