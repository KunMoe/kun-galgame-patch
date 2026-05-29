package dto

// GetMessageRequest is the request for fetching messages.
//
// max=50 to match the frontend (notice / follow / mention / etc. pages all
// fetch with limit=50). The legacy max=30 cap caused every /message GET to
// 400 on validation.
type GetMessageRequest struct {
	Type  string `query:"type"`
	Page  int    `query:"page" validate:"required,min=1"`
	Limit int    `query:"limit" validate:"required,min=1,max=50"`
}

// ReadMessageRequest is the request for marking messages as read
type ReadMessageRequest struct {
	Type string `json:"type" validate:"required,max=20"`
}
