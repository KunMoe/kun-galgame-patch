package rating

import (
	stderrors "errors"
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/gorm"
)

var ErrNotFound = stderrors.New("rating not found")

type Row struct {
	ID               int64
	WorkID           int64
	UserID           int
	Overall          int
	Art              *int
	Story            *int
	Music            *int
	Character        *int
	Route            *int
	System           *int
	Voice            *int
	ReplayValue      *int
	Recommend        string
	SpoilerLevel     string
	ShortSummary     string
	AuthorState      *string
	AuthorCompletion *string
	ClientID         string
	Site             string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func rowOf(r *catalogv2.UserRating) Row {
	a := r.Aspects
	row := Row{
		ID: r.ID, WorkID: r.WorkID, UserID: r.UserID, Overall: r.Overall,
		Art: a.Art, Story: a.Story, Music: a.Music, Character: a.Character,
		Route: a.Route, System: a.System, Voice: a.Voice, ReplayValue: a.ReplayValue,
		Recommend: r.Recommend, SpoilerLevel: r.SpoilerLevel, ShortSummary: r.ShortSummary,
		ClientID: r.ClientID, Site: r.Site,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.State != "" {
		state := r.State
		row.AuthorState, row.AuthorCompletion = &state, r.Completion
	}
	return row
}

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

const ratingColumns = `id, work_id, user_id, overall, art, story, music, "character", route, "system",
	voice, replay_value, recommend, spoiler_level, short_summary, author_state, author_completion,
	client_id, site, created_at, updated_at`

// Mirror writes NextMoe's copy of one rating and reports whether anything
// changed. An unchanged row is not rewritten: every drain replays the partial
// page after its cursor, and the write-through after a save replays it again.
// Another id on the same (user, work) is a rating that was replaced and whose
// deletion has not reached the feed yet.
func (s *Store) Mirror(r *catalogv2.UserRating) (bool, error) {
	row := rowOf(r)
	changed := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM galgame_rating WHERE user_id = ? AND work_id = ? AND id <> ?`,
			row.UserID, row.WorkID, row.ID).Error; err != nil {
			return err
		}
		res := tx.Exec(`
			INSERT INTO galgame_rating (`+ratingColumns+`)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				work_id = EXCLUDED.work_id, user_id = EXCLUDED.user_id, overall = EXCLUDED.overall,
				art = EXCLUDED.art, story = EXCLUDED.story, music = EXCLUDED.music,
				"character" = EXCLUDED."character", route = EXCLUDED.route, "system" = EXCLUDED."system",
				voice = EXCLUDED.voice, replay_value = EXCLUDED.replay_value,
				recommend = EXCLUDED.recommend, spoiler_level = EXCLUDED.spoiler_level,
				short_summary = EXCLUDED.short_summary, author_state = EXCLUDED.author_state,
				author_completion = EXCLUDED.author_completion, client_id = EXCLUDED.client_id,
				site = EXCLUDED.site, created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at
			WHERE (galgame_rating.work_id, galgame_rating.user_id, galgame_rating.overall,
				galgame_rating.art, galgame_rating.story, galgame_rating.music, galgame_rating."character",
				galgame_rating.route, galgame_rating."system", galgame_rating.voice, galgame_rating.replay_value,
				galgame_rating.recommend, galgame_rating.spoiler_level, galgame_rating.short_summary,
				galgame_rating.author_state, galgame_rating.author_completion, galgame_rating.client_id,
				galgame_rating.site, galgame_rating.created_at, galgame_rating.updated_at)
			IS DISTINCT FROM (EXCLUDED.work_id, EXCLUDED.user_id, EXCLUDED.overall,
				EXCLUDED.art, EXCLUDED.story, EXCLUDED.music, EXCLUDED."character",
				EXCLUDED.route, EXCLUDED."system", EXCLUDED.voice, EXCLUDED.replay_value,
				EXCLUDED.recommend, EXCLUDED.spoiler_level, EXCLUDED.short_summary,
				EXCLUDED.author_state, EXCLUDED.author_completion, EXCLUDED.client_id,
				EXCLUDED.site, EXCLUDED.created_at, EXCLUDED.updated_at)`,
			row.ID, row.WorkID, row.UserID, row.Overall,
			row.Art, row.Story, row.Music, row.Character, row.Route, row.System, row.Voice, row.ReplayValue,
			row.Recommend, row.SpoilerLevel, row.ShortSummary, row.AuthorState, row.AuthorCompletion,
			row.ClientID, row.Site, row.CreatedAt, row.UpdatedAt)
		changed = res.RowsAffected > 0
		return res.Error
	})
	return changed, err
}

func (s *Store) Drop(id int64) (bool, error) {
	res := s.db.Exec(`DELETE FROM galgame_rating WHERE id = ?`, id)
	return res.RowsAffected > 0, res.Error
}

func (s *Store) DropUserWork(userID int, workID int64) error {
	return s.db.Exec(`DELETE FROM galgame_rating WHERE user_id = ? AND work_id = ?`, userID, workID).Error
}

func (s *Store) Get(id int64) (Row, error) {
	var rows []Row
	if err := s.db.Raw(`SELECT `+ratingColumns+` FROM galgame_rating WHERE id = ?`, id).Scan(&rows).Error; err != nil {
		return Row{}, err
	}
	if len(rows) == 0 {
		return Row{}, ErrNotFound
	}
	return rows[0], nil
}

func (s *Store) ListByWork(workID int64, offset, limit int) ([]Row, int64, error) {
	var total int64
	if err := s.db.Raw(`SELECT COUNT(*) FROM galgame_rating WHERE work_id = ?`, workID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []Row{}
	if total == 0 {
		return rows, 0, nil
	}
	err := s.db.Raw(`SELECT `+ratingColumns+` FROM galgame_rating WHERE work_id = ?
		ORDER BY created_at DESC, id DESC OFFSET ? LIMIT ?`, workID, offset, limit).Scan(&rows).Error
	return rows, total, err
}

func (s *Store) PatchRows(ids []int) (map[int]patchModel.Patch, error) {
	out := make(map[int]patchModel.Patch, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []patchModel.Patch
	if err := s.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.ID] = p
	}
	return out, nil
}
