// Package dto holds the /admin/doc request shapes.
package dto

// DocCreateRequest is POST /admin/doc. The stored slug is "<category>/<name>";
// the client sends category + name (within-category, no slash). banner_image_hash
// is optional; status defaults to draft (0) when omitted.
type DocCreateRequest struct {
	Category        string `json:"category" validate:"required,min=1,max=64"`
	Name            string `json:"name" validate:"required,min=1,max=128"` // slug within the category
	Title           string `json:"title" validate:"required,min=1,max=255"`
	Description     string `json:"description" validate:"max=2000"`
	Content         string `json:"content" validate:"required,min=1"`
	BannerImageHash string `json:"banner_image_hash" validate:"omitempty,len=64,hexadecimal"`
	Date            string `json:"date" validate:"omitempty,datetime=2006-01-02"`
	Status          *int   `json:"status" validate:"omitempty,oneof=0 1"`
	Pin             *bool  `json:"pin"`
}

// DocUpdateRequest is PUT /admin/doc/:id. Pointer fields = presence semantics
// (nil = leave unchanged). banner_image_hash "" clears the banner.
type DocUpdateRequest struct {
	Category        *string `json:"category" validate:"omitempty,min=1,max=64"`
	Name            *string `json:"name" validate:"omitempty,min=1,max=128"`
	Title           *string `json:"title" validate:"omitempty,min=1,max=255"`
	Description     *string `json:"description" validate:"omitempty,max=2000"`
	Content         *string `json:"content" validate:"omitempty,min=1"`
	BannerImageHash *string `json:"banner_image_hash"`
	Date            *string `json:"date" validate:"omitempty,datetime=2006-01-02"`
	Status          *int    `json:"status" validate:"omitempty,oneof=0 1"`
	Pin             *bool   `json:"pin"`
}
