// Package dto holds the /blog and /admin/blog request shapes.
package dto

// BlogCreateRequest is POST /admin/blog. banner_image_hash is optional (a post
// can be published without a banner); when set it must be a 64-char image_service
// hash (validated in the service). status defaults to draft (0) when omitted.
type BlogCreateRequest struct {
	Title           string `json:"title" validate:"required,min=1,max=255"`
	Summary         string `json:"summary" validate:"max=2000"`
	Content         string `json:"content" validate:"required,min=1"`
	BannerImageHash string `json:"banner_image_hash" validate:"omitempty,len=64,hexadecimal"`
	Status          *int   `json:"status" validate:"omitempty,oneof=0 1"`
	Pin             *bool  `json:"pin"`
}

// BlogUpdateRequest is PUT /admin/blog/:id. All fields are pointers — nil means
// "leave unchanged" (presence semantics). banner_image_hash may be set to ""
// to clear the banner, or a 64-char hash to replace it.
type BlogUpdateRequest struct {
	Title           *string `json:"title" validate:"omitempty,min=1,max=255"`
	Summary         *string `json:"summary" validate:"omitempty,max=2000"`
	Content         *string `json:"content" validate:"omitempty,min=1"`
	BannerImageHash *string `json:"banner_image_hash"`
	Status          *int    `json:"status" validate:"omitempty,oneof=0 1"`
	Pin             *bool   `json:"pin"`
}
