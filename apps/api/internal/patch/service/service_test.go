package service_test

import (
	"testing"

	"kun-galgame-patch-api/internal/patch/service"

	"github.com/stretchr/testify/assert"
)

func TestExtractMentionUserIDs(t *testing.T) {
	svc := service.New(nil, nil, nil, nil, nil, nil, nil)

	tests := []struct {
		name    string
		content string
		want    []int
	}{
		{
			name:    "single mention",
			content: "Hello [@user1](/user/42/resource) check this",
			want:    []int{42},
		},
		{
			name:    "multiple mentions",
			content: "[@a](/user/1/resource) and [@b](/user/2/resource)",
			want:    []int{1, 2},
		},
		{
			name:    "duplicate mentions deduplicated",
			content: "[@a](/user/1/resource) again [@a](/user/1/resource)",
			want:    []int{1},
		},
		{
			name:    "no mentions",
			content: "just plain text",
			want:    nil,
		},
		{
			name:    "malformed mention",
			content: "[@user](/user/abc/resource)",
			want:    nil,
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.ExtractMentionUserIDs(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}
