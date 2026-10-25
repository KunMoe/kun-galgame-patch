package archive

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

type FileEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Files int    `json:"files,omitempty"`
	Items int    `json:"items,omitempty"`
}

type FileEntries []FileEntry

func (e *FileEntries) Scan(src any) error {
	*e = FileEntries{}
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, (*[]FileEntry)(e))
	case string:
		return json.Unmarshal([]byte(v), (*[]FileEntry)(e))
	default:
		return fmt.Errorf("artifact listing: cannot scan %T into entries", src)
	}
}

func (e FileEntries) Value() (driver.Value, error) {
	return json.Marshal([]FileEntry(e))
}

// State is pending until the sweeper has read the file, unsupported when it is
// not a zip (a rar or 7z the forge has not normalized yet).
type Listing struct {
	State      string      `json:"state"`
	FileCount  int         `json:"file_count"`
	DirCount   int         `json:"dir_count"`
	TotalBytes int64       `json:"total_bytes"`
	Entries    FileEntries `json:"entries"`
	Truncated  bool        `json:"truncated"`
}

// hostedResource is the condition the sweeper and both reads share: a public
// resource whose file the artifact service holds.
const hostedResource = `r.storage = 's3' AND r.artifact_uuid <> '' AND r.status = 0`

func listFiles(ctx context.Context, db *gorm.DB, resourceID int) (*Listing, error) {
	var rows []Listing
	err := db.WithContext(ctx).Raw(`
SELECT COALESCE(al.state, 'pending') AS state,
       CASE WHEN al.state = 'ready' THEN al.file_count ELSE 0 END AS file_count,
       CASE WHEN al.state = 'ready' THEN al.dir_count ELSE 0 END AS dir_count,
       CASE WHEN al.state = 'ready' THEN al.total_bytes ELSE 0 END AS total_bytes,
       CASE WHEN al.state = 'ready' THEN al.entries ELSE '[]'::jsonb END AS entries
  FROM patch_resource r
  LEFT JOIN artifact_listing al ON al.artifact_uuid = r.artifact_uuid
 WHERE r.id = ? AND `+hostedResource, resourceID).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

type fileSource struct {
	ArtifactUUID string
	Size         int64
}

// fileSource resolves only a path the stored listing holds, so a request can
// never name an entry the sweeper did not read.
func findFileSource(ctx context.Context, db *gorm.DB, resourceID int, path string) (*fileSource, error) {
	var rows []fileSource
	err := db.WithContext(ctx).Raw(`
SELECT r.artifact_uuid, (e->>'size')::bigint AS size
  FROM patch_resource r
  JOIN artifact_listing al ON al.state = 'ready' AND al.artifact_uuid = r.artifact_uuid
 CROSS JOIN LATERAL jsonb_array_elements(al.entries) e
 WHERE r.id = ? AND `+hostedResource+` AND e->>'path' = ?
 LIMIT 1`, resourceID, path).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
