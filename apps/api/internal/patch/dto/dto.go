package dto

type PatchCreateRequest struct {
	GalgameID int `json:"galgame_id" validate:"required,min=1"`
}

type PatchUpdateRequest struct {
	VndbID string `json:"vndb_id" validate:"required,max=20"`
}

type PatchResourceCreateRequest struct {
	GalgameID    int      `json:"galgame_id" validate:"required,min=1"`
	Storage      string   `json:"storage" validate:"required"`
	Name         string   `json:"name" validate:"max=300"`
	ModelName    string   `json:"model_name" validate:"max=1007"`
	ArtifactUUID string   `json:"artifact_uuid" validate:"max=64"`
	S3Key        string   `json:"s3_key" validate:"max=2048"`
	Content      string   `json:"content" validate:"max=1007"`
	Size         string   `json:"size" validate:"required"`
	Code         string   `json:"code" validate:"max=1007"`
	Password     string   `json:"password" validate:"max=1007"`
	Note         string   `json:"note" validate:"max=10007"`
	Type         []string `json:"type" validate:"required,min=1,max=10"`
	Language     []string `json:"language" validate:"required,min=1,max=10"`
	Platform     []string `json:"platform" validate:"required,min=1,max=10"`
}

type PatchResourceUpdateRequest struct {
	PatchResourceCreateRequest
	Reason string `json:"reason" validate:"max=500"`
}

type ResourceFileHistoryRequest struct {
	Page  int `query:"page" validate:"required,min=1"`
	Limit int `query:"limit" validate:"required,min=1,max=30"`
}
