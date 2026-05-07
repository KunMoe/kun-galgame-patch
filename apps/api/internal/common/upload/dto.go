package upload

// ─── Small files ──────────────────────────────────

// SmallInitRequest initializes a small-file upload.
type SmallInitRequest struct {
	GalgameID int    `json:"galgame_id" validate:"required,min=1"`
	FileName  string `json:"file_name" validate:"required,min=1,max=300"`
	FileSize  int64  `json:"file_size" validate:"required,min=1"`
}

// SmallInitResponse returns the presigned PUT URL and the pre-allocated s3_key.
type SmallInitResponse struct {
	S3Key     string `json:"s3_key"`
	UploadURL string `json:"upload_url"`
}

// SmallCompleteRequest is the verification request after a small-file upload finishes.
type SmallCompleteRequest struct {
	S3Key        string `json:"s3_key" validate:"required,min=1,max=2048"`
	DeclaredSize int64  `json:"declared_size" validate:"required,min=1"`
}

// CompleteResponse is the shared success response for all complete endpoints.
type CompleteResponse struct {
	S3Key string `json:"s3_key"`
	Size  int64  `json:"size"`
}

// ─── Multipart ──────────────────────────────────

// MultipartInitRequest initializes a large-file multipart upload.
type MultipartInitRequest struct {
	GalgameID int    `json:"galgame_id" validate:"required,min=1"`
	FileName  string `json:"file_name" validate:"required,min=1,max=300"`
	FileSize  int64  `json:"file_size" validate:"required,min=1"`
	PartCount int    `json:"part_count" validate:"required,min=1,max=10000"`
}

// MultipartInitResponse returns the uploadId and a presigned URL for every part.
type MultipartInitResponse struct {
	S3Key    string   `json:"s3_key"`
	UploadID string   `json:"upload_id"`
	PartURLs []string `json:"part_urls"` // index 0 corresponds to part_number 1
}

// UploadedPart is the ETag the client receives after uploading a part.
type UploadedPart struct {
	PartNumber int    `json:"part_number" validate:"required,min=1"`
	ETag       string `json:"etag" validate:"required,min=1"`
}

// MultipartCompleteRequest completes a multipart upload.
type MultipartCompleteRequest struct {
	S3Key        string         `json:"s3_key" validate:"required,min=1,max=2048"`
	UploadID     string         `json:"upload_id" validate:"required,min=1"`
	DeclaredSize int64          `json:"declared_size" validate:"required,min=1"`
	Parts        []UploadedPart `json:"parts" validate:"required,min=1,max=10000,dive"`
}

// MultipartAbortRequest aborts a multipart upload voluntarily.
type MultipartAbortRequest struct {
	S3Key    string `json:"s3_key" validate:"required,min=1,max=2048"`
	UploadID string `json:"upload_id" validate:"required,min=1"`
}
