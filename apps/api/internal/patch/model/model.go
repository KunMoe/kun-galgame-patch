package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

// PatchSummary is a compact projection of a patch for embedding inside other
// rows (e.g. a global comment row that wants to show "评论在 <game name>" without
// fetching the full patch). The Name field is filled by the enricher from
// Wiki, leaving this package free of Wiki/HTTP concerns.
type PatchSummary struct {
	ID     int           `json:"id"`
	VndbID string        `json:"vndb_id"`
	Banner string        `json:"banner"`
	Name   PatchSummaryName `json:"name"`
}

// PatchSummaryName mirrors the four-language KunLanguage shape but is defined
// here to avoid a model→enricher import.
type PatchSummaryName struct {
	EnUs string `json:"en-us"`
	JaJp string `json:"ja-jp"`
	ZhCn string `json:"zh-cn"`
	ZhTw string `json:"zh-tw"`
}

// renderNote is the single point where a resource's markdown note becomes HTML.
// Pulled out so RenderResourceNotes (and its single-element callers) share
// the same fallback behavior on render error.
func renderNote(src string) string {
	if src == "" {
		return ""
	}
	return markdown.MustRender(src)
}

// JSONArray represents a PostgreSQL jsonb array field
type JSONArray []string

func (j *JSONArray) Scan(value any) error {
	if value == nil {
		*j = JSONArray{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONArray: %v", value)
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	return json.Marshal(j)
}

// Patch is the core table of this project.
//
// D12 (2026-04-21): almost all galgame metadata (name / introduction / banner /
// released / content_limit / engine / alias) has moved to the Galgame Wiki.
//
// D13 (2026-05-07): patch.id is now equal to Wiki's galgame.id. Every "patch"
// in this system corresponds to exactly one galgame on the Wiki (1:1 via
// vndb_id), so duplicating that id locally was redundant. The remap migration
// (cmd/remap-patch-ids) backfills patch.id from the Wiki and drops the old
// galgame_id column. Child tables that previously had a `patch_id` FK are
// renamed to `galgame_id`.
//
// Patch now only keeps:
//   - Wiki linkage: vndb_id (required); patch.id IS the galgame_id
//   - Patch-specific data: translation type / supported languages / platforms / counts / user
//
// To display game name/banner/introduction, call Wiki /galgame/batch with patch.id directly.
type Patch struct {
	ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
	VndbID             string    `gorm:"uniqueIndex;type:varchar(107);not null" json:"vndb_id"`
	BID                *int      `gorm:"uniqueIndex" json:"bid"`
	Status             int       `gorm:"default:0" json:"status"`
	Download           int       `gorm:"default:0" json:"download"`
	View               int       `gorm:"default:0" json:"view"`
	ResourceUpdateTime time.Time `gorm:"autoCreateTime" json:"resource_update_time"`
	Type               JSONArray `gorm:"type:jsonb;default:'[]'" json:"type"`
	Language           JSONArray `gorm:"type:jsonb;default:'[]'" json:"language"`
	Platform           JSONArray `gorm:"type:jsonb;default:'[]'" json:"platform"`
	FavoriteCount      int       `gorm:"default:0" json:"favorite_count"`
	ContributeCount    int       `gorm:"default:0" json:"contribute_count"`
	CommentCount       int       `gorm:"default:0" json:"comment_count"`
	ResourceCount      int       `gorm:"default:0" json:"resource_count"`

	// Local mirror of Wiki galgame.release_date (PG `date`, day precision).
	// Populated on patch creation + a one-time backfill (A-lite sync). Drives
	// sort/filter by 发售日期 on GET /api/galgame — see migration 010 +
	// docs/galgame_wiki/00-handbook §17. Nullable: many galgames have no
	// known release date; a date-range filter auto-excludes NULL rows.
	ReleaseDate *time.Time `gorm:"type:date;index" json:"release_date"`

	// FK behavior (declared in migrations/000_baseline.up.sql, NOT enforced
	// by GORM AutoMigrate which we don't run — the `constraint:OnDelete:X`
	// tag here is documentation only):
	//
	//   patch.user_id → user(id)   ON DELETE RESTRICT
	//
	// Attempting to delete a user who still has patch rows will fail with
	// SQLSTATE 23503 (foreign_key_violation). This is intentional: patches
	// are user-authored content with downstream consequences (favorites,
	// comments, resources, moemoepoint), so the user-delete path must
	// explicitly handle (reassign / soft-delete / orphan-confirm) the
	// patches first instead of silently nuking everything via CASCADE.
	UserID  int       `gorm:"not null;constraint:OnDelete:RESTRICT" json:"user_id"`
	Created time.Time `gorm:"autoCreateTime" json:"created"`
	Updated time.Time `gorm:"autoUpdateTime" json:"updated"`

	// User is the publisher's brief, attached by the handler/service layer
	// from OAuth /users/batch (pkg/userclient). NOT a GORM relation -- after
	// the OAuth migration display fields live on the OAuth server, not the
	// local user table.
	User *PatchUser `gorm:"-" json:"user,omitempty"`
}

func (Patch) TableName() string { return "patch" }

// PatchUser is the wire shape of a user brief embedded in patch responses.
// Filled at request time from OAuth /users/batch via pkg/userclient.
//
// avatar_image_hash mirrors OAuth's `users.avatar_image_hash` — preferred over
// `avatar` by the frontend's resolveAvatarUrl once image_service is live.
// roles surfaces the OAuth role set so the UI can render an admin / mod badge
// next to a username (e.g. on comments) without an extra round-trip.
type PatchUser struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	AvatarImageHash string   `json:"avatar_image_hash"`
	Roles           []string `json:"roles,omitempty"`
}

// RenderResourceNotes fills note_html for every resource in the slice.
// Idempotent: re-rendering an already-rendered slice is a no-op rerender.
// Defined here (alongside the model) so every consumer can call it without
// importing the patch service package.
func RenderResourceNotes(rs []PatchResource) {
	for i := range rs {
		rs[i].NoteHTML = renderNote(rs[i].Note)
	}
}

// PatchResource represents a patch resource.
//
// D10 change (2026-04-21):
//   - The legacy Hash (BLAKE3) field is renamed to Blake3; kept only for existing
//     data. New uploads always leave it "".
//   - Added S3Key: the full S3 object key, e.g. "patch/42/xk9z.../game.zip".
//     All Put/Head/Delete operations use it directly; the application no longer
//     builds paths itself.
type PatchResource struct {
	ID                    int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Storage               string    `gorm:"not null" json:"storage"`
	Name                  string    `gorm:"type:varchar(300);default:''" json:"name"`
	ModelName             string    `gorm:"type:varchar(1007);default:''" json:"model_name"`
	LocalizationGroupName string    `gorm:"type:varchar(1007);default:''" json:"localization_group_name"`
	Size                  string    `gorm:"type:varchar(107);default:''" json:"size"`
	Code                  string    `gorm:"type:varchar(1007);default:''" json:"code"`
	Password              string    `gorm:"type:varchar(1007);default:''" json:"password"`
	Note                  string    `gorm:"type:varchar(10007);default:''" json:"note"`
	Blake3                string    `gorm:"default:''" json:"blake3"`
	S3Key                 string    `gorm:"type:varchar(2048);default:''" json:"s3_key"`
	Content               string    `gorm:"default:''" json:"content"`
	Type                  JSONArray `gorm:"type:jsonb;default:'[]'" json:"type"`
	Language              JSONArray `gorm:"type:jsonb;default:'[]'" json:"language"`
	Platform              JSONArray `gorm:"type:jsonb;default:'[]'" json:"platform"`
	Download              int       `gorm:"default:0" json:"download"`
	Status                int       `gorm:"default:0" json:"status"`
	UpdateTime            time.Time `gorm:"autoCreateTime" json:"update_time"`
	LikeCount             int       `gorm:"default:0" json:"like_count"`
	UserID                int       `gorm:"not null" json:"user_id"`
	GalgameID             int       `gorm:"not null" json:"galgame_id"`
	Created               time.Time `gorm:"autoCreateTime" json:"created"`
	Updated               time.Time `gorm:"autoUpdateTime" json:"updated"`

	// Filled by the handler/service layer from OAuth /users/batch.
	User *PatchUser `gorm:"-" json:"user,omitempty"`

	// NoteHTML is the rendered Note via the markdown package.
	// Filled by the service layer before serialization; not a DB column.
	NoteHTML string `gorm:"-" json:"note_html"`

	// Patch is a compact summary of the owning patch. Populated only on the
	// global resource list (/api/resource) and a few admin views; left nil
	// when the surrounding context already identifies the patch.
	Patch *PatchSummary `gorm:"-" json:"patch,omitempty"`

	// IsLiked is populated per-request from the current user's like relation
	// (mirrors PatchComment.IsLiked). Not a DB column.
	IsLiked bool `gorm:"-" json:"is_liked"`
}

func (PatchResource) TableName() string { return "patch_resource" }

// PatchComment represents a patch comment
type PatchComment struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Content   string    `gorm:"type:varchar(10007);default:''" json:"content"`
	Edit      string    `gorm:"default:''" json:"edit"`
	LikeCount int       `gorm:"default:0" json:"like_count"`
	ParentID  *int      `json:"parent_id"`
	UserID    int       `gorm:"not null" json:"user_id"`
	GalgameID int       `gorm:"not null" json:"galgame_id"`
	Created   time.Time `gorm:"autoCreateTime" json:"created"`
	Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`

	// Filled by the handler/service layer from OAuth /users/batch.
	User    *PatchUser     `gorm:"-" json:"user,omitempty"`
	Replies []PatchComment `gorm:"foreignKey:ParentID" json:"reply"`

	// IsLiked is populated per-request from the current user's like relation.
	// Not a DB column.
	IsLiked bool `gorm:"-" json:"is_liked"`

	// ContentHTML is the rendered Content via the markdown package
	// (with @mention support). Filled by the service layer before serialization.
	ContentHTML string `gorm:"-" json:"content_html"`

	// Patch is a compact summary of the owning patch. Populated only on the
	// global comment list (/api/comment) where the frontend wants to show
	// "评论在 <game name>"; left nil for the per-patch comment list since the
	// page already has the patch context.
	Patch *PatchSummary `gorm:"-" json:"patch,omitempty"`
}

func (PatchComment) TableName() string { return "patch_comment" }

// NOTE: PatchAlias is deprecated per D12 (2026-04-21). Game aliases are managed by Wiki /galgame/:gid/aliases.

// PatchLink represents an external link
type PatchLink struct {
	ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
	GalgameID int     `gorm:"uniqueIndex:idx_patch_link;index;not null" json:"galgame_id"`
	Name    string    `gorm:"uniqueIndex:idx_patch_link;type:varchar(233)" json:"name"`
	URL     string    `gorm:"type:varchar(1007)" json:"url"`
	Created time.Time `gorm:"autoCreateTime" json:"created"`
	Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (PatchLink) TableName() string { return "patch_link" }

// NOTE: PatchCover / PatchScreenshot are deprecated per decision D8.
// They are owned by the Galgame Wiki Service and not persisted in this project.

// Relation tables
type UserPatchFavoriteRelation struct {
	ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID  int       `gorm:"uniqueIndex:idx_user_patch_fav;not null" json:"user_id"`
	GalgameID int     `gorm:"uniqueIndex:idx_user_patch_fav;not null" json:"galgame_id"`
	Created time.Time `gorm:"autoCreateTime" json:"created"`
	Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (UserPatchFavoriteRelation) TableName() string { return "user_patch_favorite_relation" }

type UserPatchContributeRelation struct {
	ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID  int       `gorm:"uniqueIndex:idx_user_patch_contrib;not null" json:"user_id"`
	GalgameID int     `gorm:"uniqueIndex:idx_user_patch_contrib;not null" json:"galgame_id"`
	Created time.Time `gorm:"autoCreateTime" json:"created"`
	Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (UserPatchContributeRelation) TableName() string { return "user_patch_contribute_relation" }

type UserPatchCommentLikeRelation struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"uniqueIndex:idx_user_comment_like;not null" json:"user_id"`
	CommentID int       `gorm:"uniqueIndex:idx_user_comment_like;not null" json:"comment_id"`
	Created   time.Time `gorm:"autoCreateTime" json:"created"`
	Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (UserPatchCommentLikeRelation) TableName() string { return "user_patch_comment_like_relation" }

type UserPatchResourceLikeRelation struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int       `gorm:"uniqueIndex:idx_user_resource_like;not null" json:"user_id"`
	ResourceID int       `gorm:"uniqueIndex:idx_user_resource_like;not null" json:"resource_id"`
	Created    time.Time `gorm:"autoCreateTime" json:"created"`
	Updated    time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (UserPatchResourceLikeRelation) TableName() string { return "user_patch_resource_like_relation" }

// PatchResourceFileHistory is the append-only audit trail for resource file
// replacements (MOYU-PR5 / M3). One row is written BEFORE each substantive
// file change in PatchService.UpdateResource (Storage / S3Key / Content
// differs from current). Pure metadata edits (note / code / type / ...) do
// NOT write a row. CASCADE on delete: history goes with the resource — see
// migrations/007_patch_resource_file_history.up.sql §rationale.
type PatchResourceFileHistory struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ResourceID int       `gorm:"not null;index:idx_prfh_resource,priority:1" json:"resource_id"`
	OldStorage string    `gorm:"type:varchar(16);not null" json:"old_storage"`
	OldS3Key   string    `gorm:"type:varchar(2048);not null;default:''" json:"old_s3_key"`
	OldBlake3  string    `gorm:"type:varchar(128);not null;default:''" json:"old_blake3"`
	OldSize    string    `gorm:"type:varchar(107);not null;default:''" json:"old_size"`
	OldContent string    `gorm:"type:text;not null;default:''" json:"old_content"`
	Reason     string    `gorm:"type:varchar(500);not null;default:''" json:"reason"`
	ActorID    int       `gorm:"not null" json:"actor_id"`
	ActorRole  int       `gorm:"not null;default:0" json:"actor_role"` // 3=admin / 2=mod / 1=user / 0=unknown
	CreatedAt  time.Time `gorm:"autoCreateTime;index:idx_prfh_resource,priority:2,sort:desc" json:"created_at"`
}

func (PatchResourceFileHistory) TableName() string { return "patch_resource_file_history" }

// NOTE: PatchTag / PatchTagRel are deprecated per D11 (2026-04-21).
// Tag metadata is owned by the Galgame Wiki; fetch it via patch.vndb_id -> Wiki /galgame/batch.
