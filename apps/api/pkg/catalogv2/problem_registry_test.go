package catalogv2

import (
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

// A code left out falls back to its status, which is right for most and wrong
// for exactly the ones this table exists for: a 403 that is moyu's own key.
func TestEveryRegistryCodeIsClassified(t *testing.T) {
	for _, code := range catalogv2test.Codes() {
		if _, ok := problemKinds[code]; !ok {
			t.Errorf("%s has no kind", code)
		}
	}
	if len(problemKinds) != len(catalogv2test.Codes()) {
		t.Errorf("%d kinds for %d registry codes", len(problemKinds), len(catalogv2test.Codes()))
	}
}
