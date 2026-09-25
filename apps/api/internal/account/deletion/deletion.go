package deletion

import (
	"context"
	"fmt"

	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

const (
	syncPageLimit = 500
	syncMaxPages  = 20
)

type Feed interface {
	Configured() bool
	DeletedUsers(ctx context.Context, cursor string, limit int) (userclient.DeletedPage, error)
}

type Store interface {
	Cursor(ctx context.Context) (string, error)
	SaveCursor(ctx context.Context, cursor string) error
	Purge(ctx context.Context, userID int) error
}

type Service struct {
	feed   Feed
	store  Store
	forget func(id uint)
}

func New(db *gorm.DB, users *userclient.Client) *Service {
	forget := func(uint) {}
	if users != nil {
		forget = users.Invalidate
	}
	return newService(users, &dbStore{db: db}, forget)
}

func newService(feed Feed, store Store, forget func(id uint)) *Service {
	if forget == nil {
		forget = func(uint) {}
	}
	return &Service{feed: feed, store: store, forget: forget}
}

func (s *Service) Configured() bool {
	return s != nil && s.feed.Configured()
}

func (s *Service) Sync(ctx context.Context) (int, error) {
	cursor, err := s.store.Cursor(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for range syncMaxPages {
		page, err := s.feed.DeletedUsers(ctx, cursor, syncPageLimit)
		if err != nil {
			return n, err
		}
		if len(page.Users) == 0 {
			return n, nil
		}
		for _, u := range page.Users {
			if err := s.store.Purge(ctx, int(u.ID)); err != nil {
				return n, fmt.Errorf("purge user %d: %w", u.ID, err)
			}
			s.forget(u.ID)
			n++
		}
		if page.NextCursor == cursor {
			return n, nil
		}
		if err := s.store.SaveCursor(ctx, page.NextCursor); err != nil {
			return n, err
		}
		cursor = page.NextCursor
	}
	return n, nil
}
