package dto

// PatchCreateRequest is the create-patch request body (D12, 2026-04-21).
//
// All game metadata (name / introduction / banner / released / content_limit / alias)
// comes from the Galgame Wiki; the client only needs to supply vndb_id. The server
// calls Wiki /galgame/check to verify and fetch the galgame_id to persist locally.
type PatchCreateRequest struct {
	VndbID string `json:"vndb_id" validate:"required,max=20"`
}

// PatchUpdateRequest: after D12, the patch itself has almost no editable fields.
// This DTO is kept only for the edge case of rebinding vndb_id (e.g. a mislinked entry).
type PatchUpdateRequest struct {
	VndbID string `json:"vndb_id" validate:"required,max=20"`
}

// GetPatchCommentRequest is the request for fetching a comment list
type GetPatchCommentRequest struct {
	Page  int `query:"page" validate:"required,min=1"`
	Limit int `query:"limit" validate:"required,min=1,max=30"`
}

// PatchCommentCreateRequest is the request body for creating a comment.
//
// GalgameID is NOT required from the body: the canonical source is the URL
// path param (/patch/:id/comment), which the handler injects into this struct
// AFTER validation runs. Marking it `required` made validation reject every
// real request (the FE only sends {content}) — commenting was fully broken.
type PatchCommentCreateRequest struct {
	GalgameID int    `json:"galgame_id" validate:"omitempty,min=1"`
	ParentID  *int   `json:"parent_id" validate:"omitempty,min=1"`
	Content  string `json:"content" validate:"required,min=1,max=10007"`
	Captcha  string `json:"captcha" validate:"max=10"`
}

// PatchCommentUpdateRequest is the request body for updating a comment
type PatchCommentUpdateRequest struct {
	Content string `json:"content" validate:"required,min=1,max=10007"`
}

// PatchResourceCreateRequest is the request body for creating a resource.
//
// D10 change (2026-04-21): the Hash (BLAKE3) field is gone.
// After uploading the S3 resource, the frontend receives s3_key (full object key)
// and submits it here; the server verifies via HeadObject.
//
// Content semantics by storage type:
//   - storage="s3"   : frontend may leave Content empty; the service overwrites
//                      it with s3_key. Download is materialized at fetch time
//                      by GetResourceDownloadInfo (S3Client.PublicURL + s3_key)
//                      so the bucket's public domain can change without DB
//                      backfill. validate has no required/min so the FE doesn't
//                      have to send a placeholder string just to pass schema.
//   - storage="user" : frontend supplies the user's own download links here,
//                      comma-separated. min=1 is enforced at the service layer
//                      below for this branch.
type PatchResourceCreateRequest struct {
	GalgameID int      `json:"galgame_id" validate:"required,min=1"`
	Storage   string   `json:"storage" validate:"required"`
	Name      string   `json:"name" validate:"max=300"`
	ModelName string   `json:"model_name" validate:"max=1007"`
	S3Key     string   `json:"s3_key" validate:"max=2048"`
	Content   string   `json:"content" validate:"max=1007"`
	Size      string   `json:"size" validate:"required"`
	Code      string   `json:"code" validate:"max=1007"`
	Password  string   `json:"password" validate:"max=1007"`
	Note      string   `json:"note" validate:"max=10007"`
	Type      []string `json:"type" validate:"required,min=1,max=10"`
	Language  []string `json:"language" validate:"required,min=1,max=10"`
	Platform  []string `json:"platform" validate:"required,min=1,max=10"`
}

// PatchResourceUpdateRequest is the request body for updating a resource.
// Reason is the optional "why am I replacing the file" memo — captured into
// patch_resource_file_history when the file actually changed (Storage / S3Key
// / Content differs from current). Pure metadata edits don't record history
// regardless of whether Reason was set (MOYU-PR5 / M3).
type PatchResourceUpdateRequest struct {
	PatchResourceCreateRequest
	Reason string `json:"reason" validate:"max=500"`
}

// DuplicateCheckRequest is the request for checking VNDB ID duplicates
type DuplicateCheckRequest struct {
	VndbID string `query:"vndb_id" validate:"required,max=20"`
}
