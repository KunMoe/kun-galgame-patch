package dto

type SubmitReportRequest struct {
	SubjectKind string `json:"subject_kind" validate:"required,oneof=patch_resource user"`
	SubjectID   string `json:"subject_id" validate:"required,max=64"`
	ReasonKey   string `json:"reason_key" validate:"required,max=64"`
	Note        string `json:"note" validate:"max=1000"`
	Snapshot    string `json:"snapshot" validate:"max=2000"`
	SubjectURL  string `json:"subject_url" validate:"omitempty,http_url,max=512"`
}

type SubmitReportResponse struct {
	ReportID int64 `json:"report_id"`
}

type ListReviewItemsRequest struct {
	Status int
	Source int
	Page   int
	Limit  int
}

type TrustCallback struct {
	DispositionID int64  `json:"disposition_id"`
	SubjectKind   string `json:"subject_kind"`
	SubjectID     string `json:"subject_id"`
	Action        int16  `json:"action"`
	ReasonCode    string `json:"reason_code"`
}
