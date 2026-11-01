package service

import (
	"testing"

	"kun-galgame-patch-api/internal/patch/model"

	"github.com/stretchr/testify/assert"
)

func TestShuffleUnpinnedKeepsPinsInPlace(t *testing.T) {
	one, two := 1, 2
	rs := []model.PatchResource{{ID: 10, DisplayOrder: &one}, {ID: 20, DisplayOrder: &two}}
	for id := 30; id < 80; id++ {
		rs = append(rs, model.PatchResource{ID: id})
	}

	moved := false
	for range 20 {
		shuffleUnpinned(rs)
		assert.Equal(t, 10, rs[0].ID)
		assert.Equal(t, 20, rs[1].ID)
		moved = moved || rs[2].ID != 30
	}
	assert.True(t, moved, "the unpinned tail is never shuffled")
	assert.Len(t, rs, 52)
}
