package common

import (
	"context"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func (h *CommonHandler) excludedWorkIDs(ctx context.Context, exclude catalogv2.TagExclusion) ([]int, error) {
	if exclude.Empty() {
		return nil, nil
	}
	return h.galgame.ExcludedWorkIDs(ctx, exclude)
}
