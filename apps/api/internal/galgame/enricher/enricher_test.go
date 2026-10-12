package enricher

import (
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	patchModel "kun-galgame-patch-api/internal/patch/model"
)

func intPtr(v int) *int { return &v }

func sameID(got *int, want int) bool {
	if want == 0 {
		return got == nil
	}
	return got != nil && *got == want
}

func TestExternalIDsNeverFallBackToTheLocalColumn(t *testing.T) {
	artbook := &patchModel.Patch{ID: 106, BangumiID: intPtr(78931)}

	cards := []struct {
		name         string
		brief        galgameClient.GalgameBrief
		bangumi, egs int
	}{
		{"catalog's subject wins", galgameClient.GalgameBrief{ID: 106, BangumiID: intPtr(851)}, 851, 0},
		{"no exact ref leaves it empty", galgameClient.GalgameBrief{ID: 106}, 0, 0},
	}
	for _, tc := range cards {
		t.Run("card: "+tc.name, func(t *testing.T) {
			card := CardFromBriefAndPatch(&tc.brief, artbook)
			if !sameID(card.BangumiID, tc.bangumi) || !sameID(card.ErogamescapeID, tc.egs) {
				t.Errorf("ids = (%v, %v), want (%d, %d)", card.BangumiID, card.ErogamescapeID, tc.bangumi, tc.egs)
			}
		})
	}

	details := []struct {
		name         string
		patch        *patchModel.Patch
		full         galgameClient.GalgameFull
		bangumi, egs int
	}{
		{"no exact ref leaves it empty", artbook, galgameClient.GalgameFull{ID: 106}, 0, 0},
		{
			"both ids lift onto the card",
			&patchModel.Patch{ID: 1781},
			galgameClient.GalgameFull{ID: 1781, BangumiID: intPtr(26450), ErogamescapeID: intPtr(16280)},
			26450, 16280,
		},
	}
	for _, tc := range details {
		t.Run("detail: "+tc.name, func(t *testing.T) {
			base := &PatchDetailCard{GalgameCard: baseCard(tc.patch)}
			applyCatalogEntities(base, &tc.full)
			if !sameID(base.BangumiID, tc.bangumi) || !sameID(base.ErogamescapeID, tc.egs) {
				t.Errorf("ids = (%v, %v), want (%d, %d)", base.BangumiID, base.ErogamescapeID, tc.bangumi, tc.egs)
			}
		})
	}
}
