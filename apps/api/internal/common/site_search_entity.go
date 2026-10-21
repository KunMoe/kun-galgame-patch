package common

import (
	"context"
	"log/slog"
	"sync"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/errors"
)

type searchEntityGroup struct {
	Family string                           `json:"family"`
	Total  int64                            `json:"total"`
	Items  []galgameClient.EntitySearchItem `json:"items"`
}

// searchEntityLane runs one catalog request per family. A family that fails is
// dropped so a single slow or broken one does not blank the tab, but all of
// them failing is an error rather than an empty result.
func (h *CommonHandler) searchEntityLane(
	ctx context.Context, raw, family string, page, limit int, cl string,
) ([]searchEntityGroup, *errors.AppError) {
	if h.galgame == nil {
		return nil, errors.ErrInternal("鲲 Galgame 论坛数据未启用")
	}
	families := galgameClient.EntityFamilies()
	if family != "" {
		if !galgameClient.IsEntityFamily(family) {
			return nil, errors.ErrBadRequest("未知的论坛数据类型")
		}
		families = []string{family}
	}

	groups := make([]searchEntityGroup, len(families))
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed int
	)
	for i, f := range families {
		wg.Add(1)
		go func(i int, f string) {
			// fiber's recover middleware only wraps the handler goroutine, so a
			// panic in a family would take the process down with it instead of
			// failing this one request.
			defer func() {
				if r := recover(); r != nil {
					slog.Error("论坛数据搜索 family panic", "family", f, "panic", r)
				}
			}()
			defer wg.Done()

			items, total, err := h.galgame.SearchEntities(ctx, f, raw, page, limit, cl)
			if err != nil {
				slog.Warn("论坛数据搜索 family 失败", "family", f, "error", err)
				mu.Lock()
				failed++
				mu.Unlock()
				items = []galgameClient.EntitySearchItem{}
			}
			groups[i] = searchEntityGroup{Family: f, Total: total, Items: items}
		}(i, f)
	}
	wg.Wait()

	if failed == len(families) {
		return nil, errors.ErrInternal("鲲 Galgame 论坛数据搜索失败")
	}
	return groups, nil
}
