package rating

import (
	"context"
	stderrors "errors"
	"log/slog"
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/trust/gate"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/userclient"
)

var (
	ErrContentRejected = stderrors.New("rating summary refused by the trust check")
	ErrNotAuthor       = stderrors.New("only the author or an admin may delete a rating")
)

// Catalog stores a playtime report under this floor but leaves it out of every
// median, so the form reads one as no playtime.
const playtimeFloor = 10

type Catalog interface {
	WorkRecord(ctx context.Context, accessToken string, workID int64) (*catalogv2.WorkRecord, error)
	PutWorkRecord(ctx context.Context, accessToken string, workID int64, in catalogv2.WorkRecordInput) (*catalogv2.WorkRecord, error)
	DeleteMyRating(ctx context.Context, accessToken string, workID int64) error
	ModerationDeleteRating(ctx context.Context, accessToken string, ratingID int64) error
}

type ratingStore interface {
	Mirror(r *catalogv2.UserRating) (bool, error)
	Drop(id int64) (bool, error)
	DropUserWork(userID int, workID int64) error
	Get(id int64) (Row, error)
	ListByWork(workID int64, offset, limit int) ([]Row, int64, error)
}

type Service struct {
	store   ratingStore
	catalog Catalog
	users   *userclient.Client
	gate    *gate.Gate
}

func NewService(store ratingStore, catalog Catalog, users *userclient.Client, g *gate.Gate) *Service {
	return &Service{store: store, catalog: catalog, users: users, gate: g}
}

type Item struct {
	ID           int64                   `json:"id"`
	WorkID       int64                   `json:"work_id"`
	User         *patchModel.PatchUser   `json:"user"`
	Overall      int                     `json:"overall"`
	AspectScores catalogv2.RatingAspects `json:"aspect_scores"`
	Recommend    string                  `json:"recommend"`
	SpoilerLevel string                  `json:"spoiler_level"`
	ShortSummary string                  `json:"short_summary"`
	PlayState    string                  `json:"play_state"`
	Created      time.Time               `json:"created"`
	Updated      time.Time               `json:"updated"`
	CanDelete    bool                    `json:"can_delete"`
}

func itemOf(r Row) Item {
	state := ""
	if r.AuthorState != nil {
		state = fromCatalogState(*r.AuthorState, r.AuthorCompletion)
	}
	return Item{
		ID: r.ID, WorkID: r.WorkID, Overall: r.Overall,
		AspectScores: catalogv2.RatingAspects{
			Art: r.Art, Story: r.Story, Music: r.Music, Character: r.Character,
			Route: r.Route, System: r.System, Voice: r.Voice, ReplayValue: r.ReplayValue,
		},
		Recommend: r.Recommend, SpoilerLevel: r.SpoilerLevel, ShortSummary: r.ShortSummary,
		PlayState: state, Created: r.CreatedAt, Updated: r.UpdatedAt,
	}
}

type Viewer struct {
	ID    int
	Admin bool
}

func (v Viewer) canDelete(r Row) bool {
	return v.ID > 0 && (v.ID == r.UserID || v.Admin)
}

// List leaves out the ratings of authors OAuth marks banned or deleted; the
// total still counts them.
func (s *Service) List(ctx context.Context, workID int64, viewer Viewer, offset, limit int) ([]Item, int64, error) {
	rows, total, err := s.store.ListByWork(workID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, ids)
	items := make([]Item, 0, len(rows))
	for _, r := range rows {
		brief := briefs[r.UserID]
		if brief != nil && brief.Status != 0 {
			continue
		}
		item := itemOf(r)
		item.User = patchModel.NewPatchUser(brief)
		item.CanDelete = viewer.canDelete(r)
		items = append(items, item)
	}
	return items, total, nil
}

type Record struct {
	PlayState string `json:"play_state"`
	Minutes   int    `json:"minutes"`
	Rating    *Item  `json:"rating"`
}

func recordOf(rec *catalogv2.WorkRecord) *Record {
	out := &Record{PlayState: fromCatalogState(rec.State, rec.Completion)}
	if rec.Minutes >= playtimeFloor {
		out.Minutes = rec.Minutes
	}
	if rec.Rating != nil {
		item := itemOf(rowOf(rec.Rating))
		item.CanDelete = true
		out.Rating = &item
	}
	return out
}

func (s *Service) Record(ctx context.Context, token string, workID int64) (*Record, error) {
	rec, err := s.catalog.WorkRecord(ctx, token, workID)
	if err != nil {
		return nil, err
	}
	// A rating written on another site minutes ago may not have reached the
	// mirror yet; the form shows it, so the list should too.
	if rec.Rating != nil {
		s.writeThrough(rec.Rating)
	}
	return recordOf(rec), nil
}

type RecordInput struct {
	PlayState string
	Minutes   int
	Rating    *catalogv2.RatingInput
}

// PutRecord writes the play state, this site's playtime report and the rating
// in one NextMoe transaction. A part left empty is deleted upstream, which is
// how the form removes a rating or withdraws its playtime.
func (s *Service) PutRecord(ctx context.Context, token string, userID int, workID int64, in RecordInput) (*Record, error) {
	put := catalogv2.WorkRecordInput{Rating: in.Rating}
	if in.PlayState != "" {
		state, completion, _ := toCatalogState(in.PlayState)
		put.WorkState = &catalogv2.WorkStateInput{State: state, Completion: completion}
	}
	if in.Minutes > 0 {
		put.Playtime = &catalogv2.PlaytimeInput{Minutes: in.Minutes}
	}

	summaryChanged, checkSkipped := false, false
	if in.Rating != nil && in.Rating.ShortSummary != "" {
		before, err := s.catalog.WorkRecord(ctx, token, workID)
		if err != nil {
			return nil, err
		}
		summaryChanged = before.Rating == nil || before.Rating.ShortSummary != in.Rating.ShortSummary
		if summaryChanged {
			var denied bool
			denied, checkSkipped = s.gate.Denied(ctx, gate.SubjectKindGalgameRating, in.Rating.ShortSummary, userID)
			if denied {
				return nil, ErrContentRejected
			}
		}
	}

	after, err := s.catalog.PutWorkRecord(ctx, token, workID, put)
	if err != nil {
		return nil, err
	}
	if after.Rating != nil {
		s.writeThrough(after.Rating)
		if summaryChanged {
			s.gate.ScanBg(gate.SubjectKindGalgameRating, after.Rating.ID, after.Rating.ShortSummary, userID, checkSkipped)
		}
	} else if err := s.store.DropUserWork(userID, workID); err != nil {
		slog.Error("rating record: local copy not dropped; the mirror job will drop it",
			"user_id", userID, "work_id", workID, "error", err)
	}
	return recordOf(after), nil
}

// Delete removes the rating from every NextMoe site. NextMoe judges
// catalog.rating.moderate on the token itself, and its PERMISSION_REQUIRED
// reaches the caller as it is.
func (s *Service) Delete(ctx context.Context, token string, viewer Viewer, ratingID int64) error {
	r, err := s.store.Get(ratingID)
	if err != nil {
		return err
	}
	switch {
	case r.UserID == viewer.ID:
		err = s.catalog.DeleteMyRating(ctx, token, r.WorkID)
	case viewer.Admin:
		err = s.catalog.ModerationDeleteRating(ctx, token, r.ID)
		if catalogv2.IsNotFound(err) {
			err = nil
		}
	default:
		return ErrNotAuthor
	}
	if err != nil {
		return err
	}
	if _, err := s.store.Drop(r.ID); err != nil {
		slog.Error("rating delete: local copy not dropped; the mirror job will drop it", "rating_id", r.ID, "error", err)
	}
	return nil
}

// writeThrough lets the author read their own write at once; the mirror job
// brings the same row within minutes, so a failure here is only logged.
func (s *Service) writeThrough(r *catalogv2.UserRating) {
	if _, err := s.store.Mirror(r); err != nil {
		slog.Error("rating write-through failed; the mirror job will bring it", "rating_id", r.ID, "error", err)
	}
}
