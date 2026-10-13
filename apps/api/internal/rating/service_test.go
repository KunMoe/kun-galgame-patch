package rating

import (
	"context"
	"errors"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/trust/gate"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/trustclient"
	"kun-galgame-patch-api/pkg/upstream"
)

type fakeCatalog struct {
	record     *catalogv2.WorkRecord
	puts       []catalogv2.WorkRecordInput
	deletedFor []int64
	moderated  []int64
	modErr     error
}

func (f *fakeCatalog) WorkRecord(context.Context, string, int64) (*catalogv2.WorkRecord, error) {
	return f.record, nil
}

func (f *fakeCatalog) PutWorkRecord(_ context.Context, _ string, workID int64, in catalogv2.WorkRecordInput) (*catalogv2.WorkRecord, error) {
	f.puts = append(f.puts, in)
	out := &catalogv2.WorkRecord{}
	if in.WorkState != nil {
		out.State = in.WorkState.State
	}
	if in.Rating != nil {
		out.Rating = &catalogv2.UserRating{ID: 501, WorkID: workID, UserID: 2, Overall: in.Rating.Overall,
			Recommend: in.Rating.Recommend, ShortSummary: in.Rating.ShortSummary}
	}
	return out, nil
}

func (f *fakeCatalog) DeleteMyRating(_ context.Context, _ string, workID int64) error {
	f.deletedFor = append(f.deletedFor, workID)
	return nil
}

func (f *fakeCatalog) ModerationDeleteRating(_ context.Context, _ string, id int64) error {
	f.moderated = append(f.moderated, id)
	return f.modErr
}

type fakeStore struct {
	rows         map[int64]Row
	mirrored     []int64
	dropped      []int64
	droppedWorks []int64
}

func (f *fakeStore) Mirror(r *catalogv2.UserRating) (bool, error) {
	f.mirrored = append(f.mirrored, r.ID)
	return true, nil
}

func (f *fakeStore) Drop(id int64) (bool, error) {
	f.dropped = append(f.dropped, id)
	return true, nil
}

func (f *fakeStore) DropUserWork(_ int, workID int64) error {
	f.droppedWorks = append(f.droppedWorks, workID)
	return nil
}

func (f *fakeStore) Get(id int64) (Row, error) {
	r, ok := f.rows[id]
	if !ok {
		return Row{}, ErrNotFound
	}
	return r, nil
}

func (f *fakeStore) ListByWork(int64, int, int) ([]Row, int64, error) {
	return nil, 0, nil
}

type fakeTrust struct {
	decision string
	checked  []string
	scans    chan trustclient.ScanRequest
}

func (f *fakeTrust) Configured() bool { return true }

func (f *fakeTrust) Check(_ context.Context, req trustclient.CheckRequest) (*trustclient.CheckResult, error) {
	f.checked = append(f.checked, req.Text)
	return &trustclient.CheckResult{Decision: f.decision}, nil
}

func (f *fakeTrust) Scan(_ context.Context, req trustclient.ScanRequest) (*trustclient.ScanResult, error) {
	f.scans <- req
	return &trustclient.ScanResult{}, nil
}

func newTestService(cat *fakeCatalog, st *fakeStore, trust *fakeTrust) *Service {
	return NewService(st, cat, nil, gate.New(trust))
}

func newTrust(decision string) *fakeTrust {
	return &fakeTrust{decision: decision, scans: make(chan trustclient.ScanRequest, 4)}
}

func TestPutRecordRefusedSummaryWritesNothing(t *testing.T) {
	cat := &fakeCatalog{record: &catalogv2.WorkRecord{}}
	st := &fakeStore{}
	svc := newTestService(cat, st, newTrust(gate.DecisionDeny))

	_, err := svc.PutRecord(context.Background(), "tok", 2, 58, RecordInput{
		PlayState: DoneAll,
		Rating:    &catalogv2.RatingInput{Overall: 7, Recommend: "yes", ShortSummary: "bad words"},
	})
	if !errors.Is(err, ErrContentRejected) {
		t.Fatalf("err = %v, want ErrContentRejected", err)
	}
	if len(cat.puts) != 0 || len(st.mirrored) != 0 {
		t.Fatalf("a refused summary still wrote: puts %d, mirrored %v", len(cat.puts), st.mirrored)
	}
}

func TestPutRecordChecksAndScansOnlyAChangedSummary(t *testing.T) {
	ctx := context.Background()
	in := RecordInput{PlayState: DoneMain, Rating: &catalogv2.RatingInput{Overall: 8, Recommend: "yes", ShortSummary: "好玩"}}

	unchanged := newTrust(gate.DecisionAllow)
	cat := &fakeCatalog{record: &catalogv2.WorkRecord{Rating: &catalogv2.UserRating{ID: 501, ShortSummary: "好玩"}}}
	if _, err := newTestService(cat, &fakeStore{}, unchanged).PutRecord(ctx, "tok", 2, 58, in); err != nil {
		t.Fatalf("PutRecord: %v", err)
	}
	if len(unchanged.checked) != 0 || len(cat.puts) != 1 {
		t.Fatalf("checked %v, puts %d; want no check and one write for an unchanged summary", unchanged.checked, len(cat.puts))
	}
	select {
	case req := <-unchanged.scans:
		t.Fatalf("an unchanged summary was scanned again: %+v", req)
	default:
	}

	changed := newTrust(gate.DecisionAllow)
	cat = &fakeCatalog{record: &catalogv2.WorkRecord{}}
	st := &fakeStore{}
	if _, err := newTestService(cat, st, changed).PutRecord(ctx, "tok", 2, 58, in); err != nil {
		t.Fatalf("PutRecord: %v", err)
	}
	if len(changed.checked) != 1 {
		t.Fatalf("checked %v, want the new summary checked once", changed.checked)
	}
	req := <-changed.scans
	if req.SubjectKind != gate.SubjectKindGalgameRating || req.SubjectID != "501" || req.Text != "好玩" {
		t.Errorf("scan = %+v, want the central rating id", req)
	}
	if put := cat.puts[0]; put.WorkState == nil || put.WorkState.State != "done" || put.WorkState.Completion != "main" {
		t.Errorf("work_state = %+v, want done/main", put.WorkState)
	}
	if len(st.mirrored) != 1 || st.mirrored[0] != 501 {
		t.Errorf("mirrored = %v, want the saved rating written through", st.mirrored)
	}
}

// A part the form leaves empty is deleted upstream; the local copy follows.
func TestPutRecordWithoutRatingDropsIt(t *testing.T) {
	cat := &fakeCatalog{record: &catalogv2.WorkRecord{}}
	st := &fakeStore{}
	rec, err := newTestService(cat, st, newTrust(gate.DecisionAllow)).
		PutRecord(context.Background(), "tok", 2, 58, RecordInput{PlayState: Wish})
	if err != nil {
		t.Fatalf("PutRecord: %v", err)
	}
	put := cat.puts[0]
	if put.Rating != nil || put.Playtime != nil {
		t.Errorf("put = %+v, want rating and playtime absent", put)
	}
	if len(st.droppedWorks) != 1 || st.droppedWorks[0] != 58 {
		t.Errorf("dropped works = %v, want 58", st.droppedWorks)
	}
	if rec.PlayState != Wish || rec.Rating != nil {
		t.Errorf("record = %+v", rec)
	}
}

func TestRecordReadsPlaytimeUnderTheFloorAsNone(t *testing.T) {
	cat := &fakeCatalog{record: &catalogv2.WorkRecord{State: "done", Minutes: playtimeFloor - 1}}
	rec, err := newTestService(cat, &fakeStore{}, newTrust(gate.DecisionAllow)).Record(context.Background(), "tok", 58)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if rec.Minutes != 0 || rec.PlayState != Done {
		t.Errorf("record = %+v, want no minutes and the read-only done", rec)
	}
}

func TestDeleteTakesTheAuthorOrTheModerationPath(t *testing.T) {
	ctx := context.Background()
	rows := map[int64]Row{501: {ID: 501, WorkID: 58, UserID: 2}}

	cat, st := &fakeCatalog{}, &fakeStore{rows: rows}
	if err := newTestService(cat, st, nil).Delete(ctx, "tok", Viewer{ID: 2}, 501); err != nil {
		t.Fatalf("author delete: %v", err)
	}
	if len(cat.deletedFor) != 1 || cat.deletedFor[0] != 58 || len(cat.moderated) != 0 {
		t.Errorf("author delete went to %v / %v, want /v2/me/ratings/58", cat.deletedFor, cat.moderated)
	}

	cat, st = &fakeCatalog{}, &fakeStore{rows: rows}
	if err := newTestService(cat, st, nil).Delete(ctx, "tok", Viewer{ID: 3}, 501); !errors.Is(err, ErrNotAuthor) {
		t.Fatalf("stranger delete: %v, want ErrNotAuthor", err)
	}
	if len(cat.deletedFor)+len(cat.moderated) != 0 || len(st.dropped) != 0 {
		t.Error("a refused delete still reached catalog or the mirror")
	}

	refusal := &upstream.Error{Service: "catalog", Kind: upstream.Rejected, Status: 403, Code: "PERMISSION_REQUIRED"}
	cat, st = &fakeCatalog{modErr: refusal}, &fakeStore{rows: rows}
	err := newTestService(cat, st, nil).Delete(ctx, "tok", Viewer{ID: 3, Moderator: true}, 501)
	if !errors.Is(err, refusal) || len(st.dropped) != 0 {
		t.Fatalf("refused moderation: err %v, dropped %v; want the refusal and the row kept", err, st.dropped)
	}

	gone := &upstream.Error{Service: "catalog", Kind: upstream.NotFound, Status: 404, Code: "NOT_FOUND"}
	cat, st = &fakeCatalog{modErr: gone}, &fakeStore{rows: rows}
	if err := newTestService(cat, st, nil).Delete(ctx, "tok", Viewer{ID: 3, Moderator: true}, 501); err != nil {
		t.Fatalf("moderation delete of a rating already gone upstream: %v", err)
	}
	if len(cat.moderated) != 1 || cat.moderated[0] != 501 || len(st.dropped) != 1 {
		t.Errorf("moderated %v dropped %v, want rating 501 deleted by id and dropped here", cat.moderated, st.dropped)
	}
}

func TestRecordInputValidation(t *testing.T) {
	ok := func(r *ratingRequest) recordRequest {
		return recordRequest{PlayState: DoneAll, Minutes: 600, Rating: r}
	}
	eleven := 11
	cases := []struct {
		name string
		req  recordRequest
		want string
	}{
		{"done is read-only", recordRequest{PlayState: Done}, "游玩状态"},
		{"minutes over catalog's ceiling", recordRequest{Minutes: minutesMax + 1}, "时长"},
		{"overall out of range", ok(&ratingRequest{Overall: 0, Recommend: "yes"}), "总体评分"},
		{"unknown recommend", ok(&ratingRequest{Overall: 5, Recommend: "maybe"}), "推荐"},
		{"summary over 2000 runes", ok(&ratingRequest{Overall: 5, Recommend: "yes", ShortSummary: strings.Repeat("字", 2001)}), "2000"},
		{"aspect out of range", ok(&ratingRequest{Overall: 5, Recommend: "yes", AspectScores: &catalogv2.RatingAspects{Voice: &eleven}}), "分项"},
	}
	for _, tc := range cases {
		if _, err := recordInputOf(tc.req); err == nil || !strings.Contains(err.Message, tc.want) {
			t.Errorf("%s: err = %v, want one mentioning %q", tc.name, err, tc.want)
		}
	}

	in, err := recordInputOf(ok(&ratingRequest{Overall: 5, Recommend: "yes", ShortSummary: strings.Repeat("字", 2000)}))
	if err != nil {
		t.Fatalf("a valid record was refused: %v", err)
	}
	if in.Rating.SpoilerLevel != "none" {
		t.Errorf("spoiler_level = %q, want none when left out", in.Rating.SpoilerLevel)
	}
}
