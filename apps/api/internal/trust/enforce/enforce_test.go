package enforce

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/trust/dto"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, with
// `go test -count=1 -p 1`.
func testDB(t *testing.T) (*gorm.DB, int64) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"024_trust_disposition_applied.up.sql", "042_trust_subject_watermark.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().UnixNano() / 1000
	t.Cleanup(func() {
		db.Exec("DELETE FROM trust_disposition_applied WHERE disposition_id BETWEEN ? AND ?", base, base+100)
		db.Exec("DELETE FROM trust_subject_watermark WHERE subject_id = ?", strconv.FormatInt(base, 10))
	})
	return db, base
}

type recorder struct {
	hides, removes, restores atomic.Int32
	failHide, failRemove     atomic.Bool
	delay                    time.Duration
}

func (r *recorder) registry() Registry {
	return Registry{"patch_resource": {
		Hide: func(context.Context, int) error {
			r.hides.Add(1)
			if r.failHide.Load() {
				return errors.New("hide failed")
			}
			return nil
		},
		Remove: func(context.Context, int) error {
			time.Sleep(r.delay)
			r.removes.Add(1)
			if r.failRemove.Load() {
				return errors.New("remove failed")
			}
			return nil
		},
		Restore: func(context.Context, int) error {
			r.restores.Add(1)
			return nil
		},
	}}
}

func disposition(base, offset int64, action int16) dto.TrustCallback {
	return dto.TrustCallback{
		DispositionID: base + offset, SubjectKind: "patch_resource",
		SubjectID: strconv.FormatInt(base, 10), Action: action,
	}
}

func TestAFailedActionIsRetriedAndThenIdempotent(t *testing.T) {
	db, base := testDB(t)
	rec := &recorder{}
	svc := NewService(db, rec.registry(), nil)
	ctx := context.Background()
	remove := disposition(base, 1, ActionRemove)

	rec.failRemove.Store(true)
	if err := svc.Apply(ctx, remove); err == nil {
		t.Fatal("a failed remove must be answered as a failure so infra retries it")
	}
	rec.failRemove.Store(false)
	if err := svc.Apply(ctx, remove); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if err := svc.Apply(ctx, remove); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if got := rec.removes.Load(); got != 2 {
		t.Fatalf("remove ran %d times, want 2 (the failure and the retry)", got)
	}
}

func TestARetriedHideDoesNotUndoANewerDismiss(t *testing.T) {
	db, base := testDB(t)
	rec := &recorder{}
	svc := NewService(db, rec.registry(), nil)
	ctx := context.Background()

	rec.failHide.Store(true)
	if err := svc.Apply(ctx, disposition(base, 10, ActionHide)); err == nil {
		t.Fatal("want the first hide to fail")
	}
	rec.failHide.Store(false)
	if err := svc.Apply(ctx, disposition(base, 11, ActionNone)); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if err := svc.Apply(ctx, disposition(base, 10, ActionHide)); err != nil {
		t.Fatalf("the superseded hide must still be acknowledged: %v", err)
	}
	if got := rec.hides.Load(); got != 1 {
		t.Fatalf("hide ran %d times, want only the failed attempt", got)
	}
	if got := rec.restores.Load(); got != 1 {
		t.Fatalf("restore ran %d times", got)
	}
}

func TestConcurrentRedeliveriesApplyOnce(t *testing.T) {
	db, base := testDB(t)
	rec := &recorder{delay: 200 * time.Millisecond}
	svc := NewService(db, rec.registry(), nil)
	remove := disposition(base, 20, ActionRemove)

	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.Apply(context.Background(), remove); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := rec.removes.Load(); got != 1 {
		t.Fatalf("remove ran %d times, want 1", got)
	}
}
