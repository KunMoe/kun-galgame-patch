package inbox

import (
	"context"
	"testing"
)

func TestNilInboxIsSafe(t *testing.T) {
	var in *Inbox
	if in.Configured() {
		t.Fatal("nil Inbox is configured")
	}
	n, err := in.Sync(context.Background())
	if n != 0 || err != nil {
		t.Errorf("Sync = %d, %v", n, err)
	}
	in.MarkThreadRead(1, 7, 12)
	in.ForwardRead(context.Background(), 1, []int64{11})
}
