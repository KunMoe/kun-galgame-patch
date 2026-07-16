package dto

// SubmitReportRequest is the browser → BFF report payload. The BFF adds the
// reporter id from the session and forwards to the trust service verbatim —
// subject_kind / subject_id are asserted by the client and validated by trust,
// so the BFF stays a generic passthrough (no per-type code).
type SubmitReportRequest struct {
	SubjectKind string `json:"subject_kind" validate:"required,max=64"`
	SubjectID   string `json:"subject_id" validate:"required,max=64"`
	ReasonKey   string `json:"reason_key" validate:"required,max=64"`
	Note        string `json:"note" validate:"max=1000"`
	Snapshot    string `json:"snapshot" validate:"max=2000"`
	// SubjectURL: absolute deep-link to the content (built by the FE), so the
	// moderator console opens it in context. http(s) + ≤512 (trust also enforces).
	SubjectURL string `json:"subject_url" validate:"omitempty,http_url,max=512"`
}

// SubmitReportResponse echoes the trust report id (the browser only needs a
// success signal; the review item, if any, is internal to trust).
type SubmitReportResponse struct {
	ReportID int64 `json:"report_id"`
}

// ListReviewItemsRequest is the moderator-inbox query (populated from query
// params in the handler). Status/Source of -1 = no filter.
type ListReviewItemsRequest struct {
	Status int
	Source int
	Page   int
	Limit  int
}

// TrustCallback is the enforcement callback body posted by the trust dispatch
// worker (must match the trust service's callbackBody). disposition_id is the
// idempotency key.
type TrustCallback struct {
	DispositionID int64  `json:"disposition_id"`
	SubjectKind   string `json:"subject_kind"`
	SubjectID     string `json:"subject_id"`
	Action        int16  `json:"action"`
	ReasonCode    string `json:"reason_code"`
}
