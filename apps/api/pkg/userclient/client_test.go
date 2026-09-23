package userclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsers_CacheHitSecondCall(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeBatchResp(w, []Brief{{ID: 1, Name: "alice"}}, nil)
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	for range 3 {
		_, err := cli.Users(context.Background(), []uint{1})
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), hits.Load(), "second/third call should hit cache")
}

func TestUsers_NotFoundIsNegativeCached(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeBatchResp(w, nil, []uint{99})
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y", NotFoundTTL: time.Minute})
	for range 5 {
		out, err := cli.Users(context.Background(), []uint{99})
		require.NoError(t, err)
		assert.Empty(t, out)
	}
	assert.Equal(t, int32(1), hits.Load(), "subsequent lookups of unknown id must hit negative cache")
}

func TestUsers_NotFoundExpires(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeBatchResp(w, nil, []uint{99})
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y", NotFoundTTL: 50 * time.Millisecond})
	_, _ = cli.Users(context.Background(), []uint{99})
	time.Sleep(80 * time.Millisecond)
	_, _ = cli.Users(context.Background(), []uint{99})
	assert.Equal(t, int32(2), hits.Load())
}

func TestUsers_DedupesAndIgnoresZero(t *testing.T) {
	var captured []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, r.URL.Query().Get("ids"))
		writeBatchResp(w, []Brief{{ID: 1, Name: "alice"}, {ID: 2, Name: "bob"}}, nil)
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	out, err := cli.Users(context.Background(), []uint{0, 1, 2, 1, 0, 2})
	require.NoError(t, err)
	assert.Len(t, out, 2)
	require.Len(t, captured, 1)
	assert.Equal(t, "1,2", captured[0])
}

func TestUsers_ShardsLargeBatch(t *testing.T) {
	var batches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batches.Add(1)
		raw := r.URL.Query().Get("ids")
		var briefs []Brief
		for p := range strings.SplitSeq(raw, ",") {
			var id uint
			fmt.Sscanf(p, "%d", &id)
			briefs = append(briefs, Brief{ID: id, Name: fmt.Sprintf("u%d", id)})
		}
		writeBatchResp(w, briefs, nil)
	}))
	defer srv.Close()

	ids := make([]uint, 250)
	for i := range ids {
		ids[i] = uint(i + 1)
	}
	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	out, err := cli.Users(context.Background(), ids)
	require.NoError(t, err)
	assert.Len(t, out, 250)
	assert.Equal(t, int32(3), batches.Load())
}

func TestUsers_SingleflightCoalescesConcurrentMiss(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(80 * time.Millisecond)
		writeBatchResp(w, []Brief{{ID: 7, Name: "kun"}}, nil)
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_, err := cli.Users(context.Background(), []uint{7})
			require.NoError(t, err)
		})
	}
	wg.Wait()
	assert.Equal(t, int32(1), hits.Load(), "concurrent misses for the same id should be coalesced")
}

func TestUsers_CosmeticsDecodeAndSurviveCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"users":[
			{"id":1,"name":"alice","cosmetics":{
				"avatar_frame":{"item_id":3,"name":"sakura","static_url":"https://img/d/a.png","animated_url":"https://img/d/a.webp"},
				"profile_background":{"item_id":9,"name":"stars","static_url":"https://img/d/b.jpg"},
				"name_plate":{"item_id":12,"static_url":"https://img/d/c.png"}}},
			{"id":2,"name":"bob"}
		],"not_found":[]}}`))
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	for range 2 {
		out, err := cli.Users(context.Background(), []uint{1, 2})
		require.NoError(t, err)
		require.NotNil(t, out[1].Cosmetics)
		assert.Equal(t, &Decoration{ItemID: 3, Name: "sakura", StaticURL: "https://img/d/a.png", AnimatedURL: "https://img/d/a.webp"}, out[1].Cosmetics.AvatarFrame)
		assert.Equal(t, &Decoration{ItemID: 9, Name: "stars", StaticURL: "https://img/d/b.jpg"}, out[1].Cosmetics.ProfileBackground)
		assert.Nil(t, out[2].Cosmetics)
	}
	assert.Equal(t, int32(1), hits.Load())
}

func TestUser_ReturnsNilOnNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeBatchResp(w, nil, []uint{42})
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	u, err := cli.User(context.Background(), 42)
	require.NoError(t, err)
	assert.Nil(t, u)
}

func TestSearch_ReturnsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "kun", r.URL.Query().Get("q"))
		assert.Equal(t, "5", r.URL.Query().Get("limit"))
		writeJSONResp(w, map[string]any{
			"code": 0,
			"data": map[string]any{"users": []Brief{{ID: 1, Name: "kun"}}},
		})
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	users, err := cli.Search(context.Background(), "kun", 5)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "kun", users[0].Name)
}

func TestInvalidate_DropsPositiveAndNegativeEntries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeBatchResp(w, []Brief{{ID: 3, Name: "kun"}}, nil)
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	_, _ = cli.Users(context.Background(), []uint{3})
	cli.Invalidate(3)
	_, _ = cli.Users(context.Background(), []uint{3})
	assert.Equal(t, int32(2), hits.Load())
}

func TestNewMock_ServesFromMap(t *testing.T) {
	cli := NewMock(t, map[uint]*Brief{
		1: {ID: 1, Name: "alice"},
		2: {ID: 2, Name: "bob"},
	})
	out, err := cli.Users(context.Background(), []uint{1, 2, 99})
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "alice", out[1].Name)
	assert.Equal(t, "bob", out[2].Name)
}

func writeBatchResp(w http.ResponseWriter, users []Brief, notFound []uint) {
	writeJSONResp(w, map[string]any{
		"code": 0,
		"data": map[string]any{
			"users":     users,
			"not_found": notFound,
		},
	})
}

func writeJSONResp(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
