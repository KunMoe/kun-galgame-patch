package archive

import (
	"fmt"
	"reflect"
	"testing"
)

func TestOutlineKeepsASmallListingWhole(t *testing.T) {
	l := Listing{Entries: FileEntries{{Path: "w/"}, {Path: "w/a/b/c.txt", Size: 1}}}
	l.Outline()
	if l.Truncated || len(l.Entries) != 2 {
		t.Fatalf("outlined a small listing: %+v", l)
	}
}

func TestOutlineCutsBelowTheSecondLevel(t *testing.T) {
	entries := FileEntries{
		{Path: "w/"},
		{Path: "w/game.exe", Size: 100},
		{Path: "w/empty/"},
		{Path: "w/voice/a/"},
		{Path: "w/voice/b/"},
	}
	for i := range outlineAbove {
		entries = append(entries, FileEntry{Path: fmt.Sprintf("w/voice/a/%04d.ogg", i), Size: 2})
	}
	entries = append(entries,
		FileEntry{Path: "w/data/x/y/z.bin", Size: 7},
		FileEntry{Path: "w/data/top.bin", Size: 3},
	)
	l := Listing{Entries: entries}
	l.Outline()

	want := FileEntries{
		{Path: "w/"},
		{Path: "w/game.exe", Size: 100},
		{Path: "w/empty/"},
		{Path: "w/voice/", Size: 2 * outlineAbove, Files: outlineAbove, Items: 2},
		{Path: "w/data/", Size: 10, Files: 2, Items: 2},
	}
	if !l.Truncated || !reflect.DeepEqual(l.Entries, want) {
		t.Fatalf("outline = %+v", l.Entries)
	}
}

func TestOutlineWithoutAWrappingFolder(t *testing.T) {
	entries := FileEntries{{Path: "readme.txt", Size: 5}}
	for i := range outlineAbove {
		entries = append(entries, FileEntry{Path: fmt.Sprintf("a/b/%04d", i), Size: 1})
	}
	l := Listing{Entries: entries}
	l.Outline()
	want := FileEntries{
		{Path: "readme.txt", Size: 5},
		{Path: "a/"},
		{Path: "a/b/", Size: outlineAbove, Files: outlineAbove, Items: outlineAbove},
	}
	if !reflect.DeepEqual(l.Entries, want) {
		t.Fatalf("outline = %+v", l.Entries)
	}
}
