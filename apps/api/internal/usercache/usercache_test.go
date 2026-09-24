package usercache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb), mr
}

func counting(n *atomic.Int32, v []int64) func(context.Context) ([]int64, error) {
	return func(context.Context) ([]int64, error) {
		n.Add(1)
		return v, nil
	}
}

func TestFetchFillsOnceUntilTheTTL(t *testing.T) {
	c, mr := newCache(t)
	ctx := context.Background()
	var fills atomic.Int32
	for range 3 {
		got, err := Fetch(ctx, c.Slot(ctx, 7, "favorites", "shelf"), time.Minute, counting(&fills, []int64{285}))
		if err != nil || len(got) != 1 || got[0] != 285 {
			t.Fatalf("Fetch = %v, %v", got, err)
		}
	}
	if fills.Load() != 1 {
		t.Fatalf("fills = %d, want 1", fills.Load())
	}
	mr.FastForward(time.Minute + time.Second)
	if _, err := Fetch(ctx, c.Slot(ctx, 7, "favorites", "shelf"), time.Minute, counting(&fills, nil)); err != nil {
		t.Fatal(err)
	}
	if fills.Load() != 2 {
		t.Fatal("an expired answer was served")
	}
}

func TestAFailedFillIsNotKept(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	boom := errors.New("QUOTA_EXCEEDED")
	if _, err := Fetch(ctx, c.Slot(ctx, 7, "claims", "wizard"), time.Minute, func(context.Context) ([]int64, error) {
		return nil, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	var fills atomic.Int32
	if _, err := Fetch(ctx, c.Slot(ctx, 7, "claims", "wizard"), time.Minute, counting(&fills, nil)); err != nil {
		t.Fatal(err)
	}
	if fills.Load() != 1 {
		t.Fatal("the failure was served from the cache")
	}
}

// A fill that started before the reader's write lands in the generation the
// write retired, so it can never overwrite what the write made true.
func TestAFillFromBeforeForgetIsNeverServed(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	stale := c.Slot(ctx, 7, "favorites", "shelf")
	c.Forget(ctx, 7, "favorites")
	stale.Store(ctx, []int64{1}, time.Minute)

	got, err := Fetch(ctx, c.Slot(ctx, 7, "favorites", "shelf"), time.Minute, func(context.Context) ([]int64, error) {
		return []int64{285}, nil
	})
	if err != nil || len(got) != 1 || got[0] != 285 {
		t.Fatalf("Fetch = %v, %v; want the fresh shelf", got, err)
	}
}

func TestForgetIsPerReaderAndPerScope(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	var fills atomic.Int32
	for _, s := range []struct {
		uid   int
		scope string
	}{{7, "favorites"}, {7, "claims"}, {8, "favorites"}} {
		if _, err := Fetch(ctx, c.Slot(ctx, s.uid, s.scope, "x"), time.Minute, counting(&fills, nil)); err != nil {
			t.Fatal(err)
		}
	}
	c.Forget(ctx, 7, "favorites")
	for _, s := range []struct {
		uid   int
		scope string
	}{{7, "claims"}, {8, "favorites"}} {
		if _, err := Fetch(ctx, c.Slot(ctx, s.uid, s.scope, "x"), time.Minute, counting(&fills, nil)); err != nil {
			t.Fatal(err)
		}
	}
	if fills.Load() != 3 {
		t.Fatalf("fills = %d; forgetting one reader's favourites refilled somebody else's", fills.Load())
	}
}

// A browser restoring a row of tabs asks for the same shelf at once; they share
// one fill instead of each spending the reader's allowance.
func TestConcurrentMissesShareOneFill(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	var fills atomic.Int32
	release := make(chan struct{})
	fill := func(context.Context) ([]int64, error) {
		fills.Add(1)
		<-release
		return []int64{285}, nil
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := Fetch(ctx, c.Slot(ctx, 7, "favorites", "shelf"), time.Minute, fill); err != nil {
				t.Error(err)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if fills.Load() != 1 {
		t.Fatalf("fills = %d, want 1", fills.Load())
	}
}

func TestWithoutRedisEveryReadFills(t *testing.T) {
	var fills atomic.Int32
	for _, c := range []*Cache{nil, New(nil)} {
		ctx := context.Background()
		if _, err := Fetch(ctx, c.Slot(ctx, 7, "favorites", "shelf"), time.Minute, counting(&fills, nil)); err != nil {
			t.Fatal(err)
		}
		c.Forget(ctx, 7, "favorites")
	}
	if fills.Load() != 2 {
		t.Fatalf("fills = %d", fills.Load())
	}
}
