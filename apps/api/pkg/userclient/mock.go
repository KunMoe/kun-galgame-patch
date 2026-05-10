package userclient

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockServer is an in-process HTTP server that satisfies the OAuth
// /users/batch and /users/search contracts using an in-memory map.
type MockServer struct {
	Server *httptest.Server

	mu    sync.Mutex
	users map[uint]*Brief
}

// NewMockServer starts a test HTTP server that serves the given users.
// Caller is responsible for Close().
func NewMockServer(users map[uint]*Brief) *MockServer {
	ms := &MockServer{users: cloneUserMap(users)}
	mux := http.NewServeMux()
	mux.HandleFunc("/users/batch", ms.handleBatch)
	mux.HandleFunc("/users/search", ms.handleSearch)
	ms.Server = httptest.NewServer(mux)
	return ms
}

// NewMock returns a Client backed by an in-memory MockServer. The server is
// shut down via t.Cleanup.
func NewMock(t *testing.T, users map[uint]*Brief) *Client {
	t.Helper()
	ms := NewMockServer(users)
	t.Cleanup(ms.Server.Close)
	return New(Config{
		BaseURL:      ms.Server.URL,
		ClientID:     "test",
		ClientSecret: "test",
		CacheTTL:     5 * time.Second,
		NotFoundTTL:  500 * time.Millisecond,
	})
}

// Set replaces or adds a user.
func (m *MockServer) Set(u *Brief) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.users == nil {
		m.users = make(map[uint]*Brief)
	}
	m.users[u.ID] = u
}

func (m *MockServer) handleBatch(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Query().Get("ids"), ",")
	out := make([]*Brief, 0, len(parts))
	notFound := make([]uint, 0)
	m.mu.Lock()
	for _, p := range parts {
		id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64)
		if err != nil {
			continue
		}
		u, ok := m.users[uint(id)]
		if !ok {
			notFound = append(notFound, uint(id))
			continue
		}
		out = append(out, u)
	}
	m.mu.Unlock()
	writeMockJSON(w, map[string]any{
		"code":    0,
		"message": "OK",
		"data": map[string]any{
			"users":     out,
			"not_found": notFound,
		},
	})
}

func (m *MockServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	m.mu.Lock()
	out := make([]*Brief, 0)
	for _, u := range m.users {
		if strings.Contains(strings.ToLower(u.Name), q) {
			out = append(out, u)
		}
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > limit {
		out = out[:limit]
	}
	writeMockJSON(w, map[string]any{
		"code":    0,
		"message": "OK",
		"data":    map[string]any{"users": out},
	})
}

func writeMockJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func cloneUserMap(in map[uint]*Brief) map[uint]*Brief {
	out := make(map[uint]*Brief, len(in))
	maps.Copy(out, in)
	return out
}
