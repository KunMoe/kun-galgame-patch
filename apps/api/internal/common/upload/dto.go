package upload

// Unified server-driven upload, backed by the centralized artifact service.
//
// One flow: Init → (single PUT | parallel multipart parts) → Complete. The
// artifact service decides single-vs-multipart AND the part size; the client
// obeys whatever Init returns (server-driven — never hardcode a chunk size).
// The opaque artifact_uuid is the only identifier the client carries from
// init → complete → resource creation.

// InitRequest starts an upload.
type InitRequest struct {
	GalgameID int    `json:"galgame_id" validate:"required,min=1"`
	FileName  string `json:"file_name" validate:"required,min=1,max=300"`
	FileSize  int64  `json:"file_size" validate:"required,min=1"`
	MimeType  string `json:"mime_type" validate:"max=100"`
}

// PartURL is one presigned multipart part (present only in multipart responses).
type PartURL struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
}

// InitResponse tells the client how to upload. Single-PUT sets UploadURL;
// multipart sets PartSize + Parts (the client slices the file by PartSize and
// PUTs each slice to the matching Parts[].URL, collecting ETags).
type InitResponse struct {
	ArtifactUUID string    `json:"artifact_uuid"`
	Multipart    bool      `json:"multipart"`
	UploadURL    string    `json:"upload_url,omitempty"`
	PartSize     int64     `json:"part_size,omitempty"`
	Parts        []PartURL `json:"parts,omitempty"`
	ExpiresAt    string    `json:"expires_at"`
}

// CompletedPart is one finished multipart part the client reports back.
type CompletedPart struct {
	PartNumber int    `json:"part_number" validate:"required,min=1"`
	ETag       string `json:"etag" validate:"required,min=1"`
}

// CompleteRequest finalizes an upload. Parts is required for multipart, omitted
// for single-PUT.
type CompleteRequest struct {
	ArtifactUUID string          `json:"artifact_uuid" validate:"required,min=1,max=64"`
	DeclaredSize int64           `json:"declared_size" validate:"required,min=1"`
	Parts        []CompletedPart `json:"parts" validate:"omitempty,max=10000,dive"`
}

// CompleteResponse is the success response.
type CompleteResponse struct {
	ArtifactUUID string `json:"artifact_uuid"`
	Size         int64  `json:"size"`
}

// AbortRequest aborts an in-progress upload.
type AbortRequest struct {
	ArtifactUUID string `json:"artifact_uuid" validate:"required,min=1,max=64"`
}
