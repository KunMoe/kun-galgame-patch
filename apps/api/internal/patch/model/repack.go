package model

import "time"

type PatchResourceRepack struct {
	ResourceID         int       `gorm:"primaryKey;autoIncrement:false" json:"resource_id"`
	ArtifactUUID       string    `gorm:"column:artifact_uuid" json:"artifact_uuid"`
	SourceArtifactUUID string    `gorm:"column:source_artifact_uuid" json:"source_artifact_uuid"`
	Blake3             string    `json:"blake3"`
	Bytes              int64     `json:"bytes"`
	FileCount          int       `json:"file_count"`
	SourceBlake3       string    `gorm:"column:source_blake3" json:"source_blake3"`
	CreatedAt          time.Time `json:"created_at"`
}

func (PatchResourceRepack) TableName() string { return "patch_resource_repack" }
