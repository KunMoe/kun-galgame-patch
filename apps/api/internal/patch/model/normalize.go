package model

import "time"

type PatchResourceNormalize struct {
	SourceArtifactUUID string     `gorm:"column:source_artifact_uuid;primaryKey" json:"source_artifact_uuid"`
	ResourceID         int        `json:"resource_id"`
	State              string     `json:"state"`
	Attempts           int        `json:"attempts"`
	LeaseUntil         *time.Time `json:"lease_until"`
	ClaimedAt          *time.Time `json:"claimed_at"`
	ArtifactUUID       string     `gorm:"column:artifact_uuid" json:"artifact_uuid"`
	Blake3             string     `json:"blake3"`
	Bytes              int64      `json:"bytes"`
	FileCount          int        `json:"file_count"`
	HasArchives        bool       `json:"has_archives"`
	SourcePassword     string     `json:"-"`
	SourceSize         string     `json:"-"`
	Code               string     `json:"code"`
	Message            string     `json:"message"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (PatchResourceNormalize) TableName() string { return "patch_resource_normalize" }
