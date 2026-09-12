package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

type PatchSummary struct {
	ID                  int              `json:"id"`
	VndbID              string           `json:"vndb_id"`
	Banner              string           `json:"banner"`
	EffectiveBannerHash string           `json:"effective_banner_hash"`
	Name                PatchSummaryName `json:"name"`
}

type PatchSummaryName struct {
	EnUs string `json:"en-us"`
	JaJp string `json:"ja-jp"`
	ZhCn string `json:"zh-cn"`
	ZhTw string `json:"zh-tw"`
}

func renderNote(src string) string {
	if src == "" {
		return ""
	}
	return markdown.MustRender(src)
}

type JSONArray []string

func (j *JSONArray) Scan(value any) error {
	if value == nil {
		*j = JSONArray{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal JSONArray: %v", value)
	}
	if len(bytes) == 0 || string(bytes) == "null" {
		*j = JSONArray{}
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	return json.Marshal(j)
}

type Patch struct {
	ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
	VndbID             string    `gorm:"uniqueIndex;type:varchar(107);not null" json:"vndb_id"`
	BangumiID          *int      `gorm:"column:bangumi_id;uniqueIndex" json:"bangumi_id"`
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

	// The catalog work this game is, resolved from VndbID through the exact
	// vndb anchor. NOT equal to ID — the two id spaces are unrelated, see
	// migration 034. NULL for the `pending-<n>` placeholders, which have no
	// work and therefore cannot be favourited. Not unique either: two pages for
	// one game share a work, which is why the index is not UNIQUE.
	CatalogWorkID *int64 `gorm:"column:catalog_work_id" json:"-"`

	// Catalog's display verdict for this work, mirrored by the changes cron
	// (migration 032). NULL means not yet mirrored, and NULL PASSES the gate.
	//
	// Read-only (`->`): the cron writes it with UpdateColumn against the table,
	// and a writable field here would let any Save of a Patch built without
	// loading this column blank a mirrored verdict back to NULL.
	ContentLimit *string `gorm:"column:content_limit;->" json:"-"`

	IsStub bool `gorm:"default:false" json:"-"`

	Published bool `gorm:"default:false" json:"-"`

	CreatorID *int `gorm:"column:creator_id" json:"-"`

	ReleaseDate *time.Time `gorm:"type:date;index" json:"release_date"`

	UserID  int       `gorm:"not null;constraint:OnDelete:RESTRICT" json:"user_id"`
	Created time.Time `gorm:"autoCreateTime" json:"created"`
	Updated time.Time `gorm:"autoUpdateTime" json:"updated"`

	User *PatchUser `gorm:"-" json:"user,omitempty"`
}

func (Patch) TableName() string { return "patch" }

type PatchUser struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	AvatarImageHash string   `json:"avatar_image_hash"`
	Roles           []string `json:"roles,omitempty"`
	SiteRoles       []string `json:"site_roles,omitempty"`
}

func RenderResourceNotes(rs []PatchResource) {
	for i := range rs {
		rs[i].NoteHTML = renderNote(rs[i].Note)
	}
}

func StripResourceSecrets(rs []PatchResource) {
	for i := range rs {
		rs[i].Content = ""
		rs[i].S3Key = ""
		rs[i].Code = ""
		rs[i].Password = ""
	}
}

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
	ArtifactUUID          string    `gorm:"column:artifact_uuid;type:varchar(36);default:''" json:"artifact_uuid"`
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

	User *PatchUser `gorm:"-" json:"user,omitempty"`

	NoteHTML string `gorm:"-" json:"note_html"`

	DownloadURL string `gorm:"-" json:"download_url,omitempty"`

	Patch *PatchSummary `gorm:"-" json:"patch,omitempty"`

	IsLiked bool `gorm:"-" json:"is_liked"`

	IsFavorite bool `gorm:"-" json:"is_favorite"`
}

func (PatchResource) TableName() string { return "patch_resource" }

type PatchComment struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Content    string    `gorm:"type:varchar(10007);default:''" json:"content"`
	Edit       string    `gorm:"default:''" json:"edit"`
	LikeCount  int       `gorm:"default:0" json:"like_count"`
	Status     int       `gorm:"default:0" json:"status"`
	ParentID   *int      `json:"parent_id"`
	ResourceID *int      `json:"resource_id,omitempty"`
	UserID     int       `gorm:"not null" json:"user_id"`
	GalgameID  int       `gorm:"not null" json:"galgame_id"`
	Created    time.Time `gorm:"autoCreateTime" json:"created"`
	Updated    time.Time `gorm:"autoUpdateTime" json:"updated"`

	User    *PatchUser     `gorm:"-" json:"user,omitempty"`
	Replies []PatchComment `gorm:"foreignKey:ParentID" json:"reply"`

	IsLiked bool `gorm:"-" json:"is_liked"`

	ContentHTML string `gorm:"-" json:"content_html"`

	Patch *PatchSummary `gorm:"-" json:"patch,omitempty"`
}

func (PatchComment) TableName() string { return "patch_comment" }

type PatchLink struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	GalgameID int       `gorm:"uniqueIndex:idx_patch_link;index;not null" json:"galgame_id"`
	Name      string    `gorm:"uniqueIndex:idx_patch_link;type:varchar(233)" json:"name"`
	URL       string    `gorm:"type:varchar(1007)" json:"url"`
	Created   time.Time `gorm:"autoCreateTime" json:"created"`
	Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (PatchLink) TableName() string { return "patch_link" }

type UserPatchContributeRelation struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"uniqueIndex:idx_user_patch_contrib;not null" json:"user_id"`
	GalgameID int       `gorm:"uniqueIndex:idx_user_patch_contrib;not null" json:"galgame_id"`
	Created   time.Time `gorm:"autoCreateTime" json:"created"`
	Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`
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

type UserPatchResourceFavoriteRelation struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int       `gorm:"uniqueIndex:idx_user_resource_favorite;not null" json:"user_id"`
	ResourceID int       `gorm:"uniqueIndex:idx_user_resource_favorite;not null" json:"resource_id"`
	Created    time.Time `gorm:"autoCreateTime" json:"created"`
	Updated    time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (UserPatchResourceFavoriteRelation) TableName() string {
	return "user_patch_resource_favorite_relation"
}

type PatchResourceFileHistory struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ResourceID      int       `gorm:"not null;index:idx_prfh_resource,priority:1" json:"resource_id"`
	OldStorage      string    `gorm:"type:varchar(16);not null" json:"old_storage"`
	OldS3Key        string    `gorm:"type:varchar(2048);not null;default:''" json:"old_s3_key"`
	OldArtifactUUID string    `gorm:"column:old_artifact_uuid;type:varchar(36);not null;default:''" json:"old_artifact_uuid"`
	OldBlake3       string    `gorm:"type:varchar(128);not null;default:''" json:"old_blake3"`
	OldSize         string    `gorm:"type:varchar(107);not null;default:''" json:"old_size"`
	OldContent      string    `gorm:"type:text;not null;default:''" json:"old_content"`
	Reason          string    `gorm:"type:varchar(500);not null;default:''" json:"reason"`
	ActorID         int       `gorm:"not null" json:"actor_id"`
	ActorRole       int       `gorm:"not null;default:0" json:"actor_role"`
	CreatedAt       time.Time `gorm:"autoCreateTime;index:idx_prfh_resource,priority:2,sort:desc" json:"created_at"`
}

func (PatchResourceFileHistory) TableName() string { return "patch_resource_file_history" }

type ResourceFieldChange struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type ResourceChangeList []ResourceFieldChange

func (c *ResourceChangeList) Scan(value any) error {
	if value == nil {
		*c = ResourceChangeList{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal ResourceChangeList: %v", value)
	}
	if len(bytes) == 0 || string(bytes) == "null" {
		*c = ResourceChangeList{}
		return nil
	}
	return json.Unmarshal(bytes, c)
}

func (c ResourceChangeList) Value() (driver.Value, error) {
	if c == nil {
		return "[]", nil
	}
	return json.Marshal(c)
}

type PatchResourceRevision struct {
	ID         int64              `gorm:"primaryKey;autoIncrement" json:"id"`
	ResourceID int                `gorm:"not null;index:idx_prr_resource,priority:1" json:"resource_id"`
	Action     string             `gorm:"type:varchar(16);not null;default:'updated'" json:"action"`
	Changes    ResourceChangeList `gorm:"type:jsonb;default:'[]'" json:"changes"`
	Reason     string             `gorm:"type:varchar(500);not null;default:''" json:"reason"`
	ActorID    int                `gorm:"not null;default:0" json:"actor_id"`
	ActorRole  int                `gorm:"not null;default:0" json:"actor_role"`
	CreatedAt  time.Time          `gorm:"autoCreateTime;index:idx_prr_resource,priority:2,sort:desc" json:"created_at"`
}

func (PatchResourceRevision) TableName() string { return "patch_resource_revision" }
