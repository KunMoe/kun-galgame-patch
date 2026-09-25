package deletion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/userclient"
)

type scriptedFeed struct {
	cursors []string
	pages   []userclient.DeletedPage
	i       int
}

func (*scriptedFeed) Configured() bool { return true }

func (f *scriptedFeed) DeletedUsers(_ context.Context, cursor string, _ int) (userclient.DeletedPage, error) {
	f.cursors = append(f.cursors, cursor)
	if len(f.pages) == 0 {
		return userclient.DeletedPage{}, nil
	}
	if f.i >= len(f.pages) {
		return f.pages[len(f.pages)-1], nil
	}
	p := f.pages[f.i]
	f.i++
	return p, nil
}

type fakeStore struct {
	cursor string
	purged []int
	saved  []string
	failOn int
}

func (s *fakeStore) Cursor(context.Context) (string, error) {
	return s.cursor, nil
}

func (s *fakeStore) SaveCursor(_ context.Context, cursor string) error {
	s.saved = append(s.saved, cursor)
	s.cursor = cursor
	return nil
}

func (s *fakeStore) Purge(_ context.Context, userID int) error {
	if s.failOn != 0 && userID == s.failOn {
		return errors.New("boom")
	}
	s.purged = append(s.purged, userID)
	return nil
}

func TestSyncPurgesPageThenSavesCursor(t *testing.T) {
	feed := &scriptedFeed{pages: []userclient.DeletedPage{
		{Users: []userclient.DeletedUser{{ID: 5}, {ID: 7}}, NextCursor: "c1"},
		{Users: nil, NextCursor: "c1"},
	}}
	store := &fakeStore{}
	var forgot []uint
	s := newService(feed, store, func(id uint) { forgot = append(forgot, id) })
	n, err := s.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("purged %d, want 2", n)
	}
	if fmt.Sprint(store.purged) != "[5 7]" {
		t.Fatalf("purged %v, want [5 7]", store.purged)
	}
	if fmt.Sprint(forgot) != "[5 7]" {
		t.Fatalf("forgot %v, want [5 7]", forgot)
	}
	if len(store.saved) != 1 || store.saved[0] != "c1" {
		t.Fatalf("saved %v, want [c1]", store.saved)
	}
}

func TestSyncKeepsCursorWhenPurgeFails(t *testing.T) {
	feed := &scriptedFeed{pages: []userclient.DeletedPage{
		{Users: []userclient.DeletedUser{{ID: 5}, {ID: 7}}, NextCursor: "c1"},
	}}
	store := &fakeStore{failOn: 7}
	var forgot []uint
	s := newService(feed, store, func(id uint) { forgot = append(forgot, id) })
	n, err := s.Sync(context.Background())
	if err == nil {
		t.Fatal("expected purge error")
	}
	if !strings.Contains(err.Error(), "7") {
		t.Fatalf("error %v does not mention the failed id", err)
	}
	if n != 1 {
		t.Fatalf("purged %d, want 1", n)
	}
	if len(store.saved) != 0 {
		t.Fatalf("saved %v, want none", store.saved)
	}
	if fmt.Sprint(store.purged) != "[5]" {
		t.Fatalf("purged %v, want [5]", store.purged)
	}
	if fmt.Sprint(forgot) != "[5]" {
		t.Fatalf("forgot %v, want [5]", forgot)
	}
}

func TestSyncStartsFromStoredCursor(t *testing.T) {
	feed := &scriptedFeed{pages: []userclient.DeletedPage{
		{Users: nil, NextCursor: "c9"},
	}}
	store := &fakeStore{cursor: "c9"}
	s := newService(feed, store, nil)
	n, err := s.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("purged %d, want 0", n)
	}
	if len(feed.cursors) != 1 || feed.cursors[0] != "c9" {
		t.Fatalf("feed cursors %v, want [c9]", feed.cursors)
	}
}

func TestSyncStopsWhenCursorDoesNotMove(t *testing.T) {
	feed := &scriptedFeed{pages: []userclient.DeletedPage{
		{Users: []userclient.DeletedUser{{ID: 1}}, NextCursor: "stuck"},
	}}
	store := &fakeStore{}
	s := newService(feed, store, nil)
	_, err := s.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(feed.cursors); n > 2 {
		t.Fatalf("feed called %d times, want at most 2", n)
	}
}
