package ziprange

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestListPlainZip(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		if _, err := w.Create("work/"); err != nil {
			t.Fatal(err)
		}
		fw, err := w.Create("work/game.exe")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte("HI")); err != nil {
			t.Fatal(err)
		}
	})
	log, srv := serveRange(t, body)
	got := mustList(t, srv.Client(), srv.URL)
	if got.FileCount != 1 || got.DirCount != 1 || got.TotalBytes != 2 {
		t.Fatalf("counts = %+v", got)
	}
	if len(got.Entries) != 2 || got.Entries[0] != (Entry{Path: "work/"}) || got.Entries[1] != (Entry{Path: "work/game.exe", Size: 2}) {
		t.Fatalf("entries = %+v", got.Entries)
	}
	hdrs := log.headers()
	assertListingRequests(t, hdrs)
	if len(hdrs) != 1 {
		t.Fatalf("requests = %d, want 1", len(hdrs))
	}
}

func TestListWithoutDirectoryEntries(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		for _, name := range []string{"pack/a.txt", "pack/b.txt", "pack/sub/c.txt"} {
			if _, err := w.Create(name); err != nil {
				t.Fatal(err)
			}
		}
	})
	_, srv := serveRange(t, body)
	got := mustList(t, srv.Client(), srv.URL)
	if got.FileCount != 3 || got.DirCount != 2 || got.TotalBytes != 0 {
		t.Fatalf("counts = %+v", got)
	}
	if len(got.Entries) != 3 {
		t.Fatalf("entries = %+v", got.Entries)
	}
	for _, e := range got.Entries {
		if strings.HasSuffix(e.Path, "/") {
			t.Fatalf("explicit directory entry %q", e.Path)
		}
	}
	if got.Entries[0].Path != "pack/a.txt" || got.Entries[1].Path != "pack/b.txt" || got.Entries[2].Path != "pack/sub/c.txt" {
		t.Fatalf("order = %+v", got.Entries)
	}
}

func TestListZip64EntrySize(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		h := &zip.FileHeader{Name: "big.bin", Method: zip.Store}
		h.UncompressedSize64 = 1 << 32
		h.CompressedSize64 = 0
		if _, err := w.CreateRaw(h); err != nil {
			t.Fatal(err)
		}
	})
	_, srv := serveRange(t, body)
	got := mustList(t, srv.Client(), srv.URL)
	if got.FileCount != 1 || got.DirCount != 0 || got.TotalBytes != 1<<32 {
		t.Fatalf("counts = %+v", got)
	}
	if len(got.Entries) != 1 || got.Entries[0].Size != 1<<32 || got.Entries[0].Path != "big.bin" {
		t.Fatalf("entries = %+v", got.Entries)
	}
}

func TestListCentralDirectorySpansChunks(t *testing.T) {
	const n = 6000
	first := longName(0)
	body := buildZip(t, func(w *zip.Writer) {
		for i := range n {
			fw, err := w.Create(longName(i))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := fw.Write([]byte{0}); err != nil {
				t.Fatal(err)
			}
		}
	})
	log, srv := serveRange(t, body)
	got := mustList(t, srv.Client(), srv.URL)
	hdrs := log.headers()
	assertListingRequests(t, hdrs)
	if len(hdrs) < 2 {
		t.Fatalf("requests = %d, want at least 2 for a central directory larger than %d bytes", len(hdrs), chunkBytes)
	}
	if got.FileCount != n {
		t.Fatalf("file count = %d", got.FileCount)
	}
	found := false
	for _, e := range got.Entries {
		if e.Path == first {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("first name %q missing", first)
	}
}

func TestListRangeIgnoredDoesNotReadBody(t *testing.T) {
	var reads atomic.Int64
	var log reqLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Length", strconv.Itoa(8<<20))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	base := srv.Client().Transport
	hc := &http.Client{Transport: &countTransport{base: base, reads: &reads}}
	_, err := list(t, hc, srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrUnsupported) {
		t.Fatalf("ignored range = %v, want a failure other than unsupported", err)
	}
	if reads.Load() != 0 {
		t.Fatalf("body reads = %d, want 0", reads.Load())
	}
	hdrs := log.headers()
	assertListingRequests(t, hdrs)
	if len(hdrs) != wholeObjectAttempts {
		t.Fatalf("requests = %d, want %d", len(hdrs), wholeObjectAttempts)
	}
}

func TestListRetriesAFirstWholeObjectAnswer(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		if _, err := w.Create("work/game.exe"); err != nil {
			t.Fatal(err)
		}
	})
	_, ranged := serveRange(t, body)
	var log reqLog
	var served atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		if served.Add(1) == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(8<<20))
			w.WriteHeader(http.StatusOK)
			return
		}
		ranged.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	got := mustList(t, srv.Client(), srv.URL)
	if got.FileCount != 1 {
		t.Fatalf("counts = %+v", got)
	}
	if n := len(log.headers()); n != 2 {
		t.Fatalf("requests = %d, want 2", n)
	}
}

func TestListTinyWholeBody(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		if _, err := w.Create("only.txt"); err != nil {
			t.Fatal(err)
		}
	})
	if len(body) > chunkBytes {
		t.Fatalf("fixture is %d bytes", len(body))
	}
	var log reqLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(body); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	got := mustList(t, srv.Client(), srv.URL)
	if got.FileCount != 1 || len(got.Entries) != 1 || got.Entries[0].Path != "only.txt" {
		t.Fatalf("listing = %+v", got)
	}
	assertListingRequests(t, log.headers())
}

func TestListNonZip(t *testing.T) {
	_, srv := serveRange(t, bytes.Repeat([]byte("this is not a zip archive."), 8))
	_, err := list(t, srv.Client(), srv.URL)
	if !errors.Is(err, ErrUnsupported) || !errors.Is(err, zip.ErrFormat) {
		t.Fatalf("err = %v, want ErrUnsupported wrapping zip.ErrFormat", err)
	}
}

func TestListNormalizesNames(t *testing.T) {
	rawName := "work/" + string([]byte{0xff, 0xfe}) + ".txt"
	body := buildZip(t, func(w *zip.Writer) {
		if _, err := w.Create(`work\game.exe`); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Create("./readme.txt"); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Create("/.//nested/x.txt"); err != nil {
			t.Fatal(err)
		}
		h := &zip.FileHeader{Name: rawName, Method: zip.Store}
		if _, err := w.CreateRaw(h); err != nil {
			t.Fatal(err)
		}
	})
	_, srv := serveRange(t, body)
	got := mustList(t, srv.Client(), srv.URL)
	want := []string{"work/game.exe", "readme.txt", "nested/x.txt", "work/\uFFFD.txt"}
	if len(got.Entries) != len(want) {
		t.Fatalf("entries = %+v", got.Entries)
	}
	for i, path := range want {
		if got.Entries[i].Path != path {
			t.Errorf("entry %d = %q, want %q", i, got.Entries[i].Path, path)
		}
	}
}

func TestListInsecurePathIsUsable(t *testing.T) {
	t.Setenv("GODEBUG", "zipinsecurepath=0")
	body := buildZip(t, func(w *zip.Writer) {
		if _, err := w.Create("../secret.txt"); err != nil {
			t.Fatal(err)
		}
	})
	_, srv := serveRange(t, body)
	got := mustList(t, srv.Client(), srv.URL)
	if got.FileCount != 1 || got.DirCount != 1 || len(got.Entries) != 1 || got.Entries[0].Path != "../secret.txt" {
		t.Fatalf("listing = %+v", got)
	}
}

func TestListEntryLimit(t *testing.T) {
	old := maxListingEntries
	maxListingEntries = 1
	t.Cleanup(func() { maxListingEntries = old })
	body := buildZip(t, func(w *zip.Writer) {
		if _, err := w.Create("a.txt"); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Create("b.txt"); err != nil {
			t.Fatal(err)
		}
	})
	_, srv := serveRange(t, body)
	_, err := list(t, srv.Client(), srv.URL)
	if !errors.Is(err, ErrUnsupported) || errors.Is(err, zip.ErrFormat) {
		t.Fatalf("err = %v", err)
	}
}

func TestListDirectoryByteLimit(t *testing.T) {
	old := maxDirectoryBytes
	maxDirectoryBytes = 64
	t.Cleanup(func() { maxDirectoryBytes = old })
	body := buildZip(t, func(w *zip.Writer) {
		fw, err := w.Create("work/game.exe")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(bytes.Repeat([]byte("x"), 80)); err != nil {
			t.Fatal(err)
		}
	})
	if len(body) <= 64 {
		t.Fatalf("fixture is %d bytes, want more than the lowered limit", len(body))
	}
	_, srv := serveRange(t, body)
	_, err := list(t, srv.Client(), srv.URL)
	if !errors.Is(err, ErrUnsupported) || errors.Is(err, zip.ErrFormat) {
		t.Fatalf("err = %v", err)
	}
}

func buildZip(t *testing.T, fn func(*zip.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fn(w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func longName(i int) string {
	return strings.Repeat("n", 190) + fmt.Sprintf("-%05d.txt", i)
}

func list(t *testing.T, hc *http.Client, rawURL string) (Listing, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return List(ctx, hc, rawURL)
}

func mustList(t *testing.T, hc *http.Client, rawURL string) Listing {
	t.Helper()
	got, err := list(t, hc, rawURL)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return got
}

type reqLog struct {
	mu   sync.Mutex
	hdrs []http.Header
}

func (l *reqLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hdrs = append(l.hdrs, r.Header.Clone())
}

func (l *reqLog) headers() []http.Header {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]http.Header, len(l.hdrs))
	copy(out, l.hdrs)
	return out
}

func serveRange(t *testing.T, body []byte) (*reqLog, *httptest.Server) {
	t.Helper()
	log := &reqLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		start, end, ok := testRange(r.Header.Get("Range"), int64(len(body)))
		if !ok {
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		if _, err := w.Write(body[start : end+1]); err != nil {
			t.Errorf("write range: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return log, srv
}

func testRange(spec string, size int64) (start, end int64, ok bool) {
	rest, found := strings.CutPrefix(spec, "bytes=")
	if !found || size <= 0 {
		return 0, 0, false
	}
	if strings.HasPrefix(rest, "-") {
		n, err := strconv.ParseInt(rest[1:], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n >= size {
			return 0, size - 1, true
		}
		return size - n, size - 1, true
	}
	a, b, found := strings.Cut(rest, "-")
	if !found {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	end, err = strconv.ParseInt(b, 10, 64)
	if err != nil || start < 0 || end < start || start >= size {
		return 0, 0, false
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
}

func assertListingRequests(t *testing.T, hdrs []http.Header) {
	t.Helper()
	if len(hdrs) == 0 {
		t.Fatal("no requests")
	}
	for i, h := range hdrs {
		if h.Get("Accept-Encoding") != "identity" {
			t.Errorf("request %d Accept-Encoding = %q", i, h.Get("Accept-Encoding"))
		}
		if h.Get("User-Agent") != userAgent {
			t.Errorf("request %d User-Agent = %q", i, h.Get("User-Agent"))
		}
	}
	want := "bytes=-" + strconv.Itoa(chunkBytes)
	if hdrs[0].Get("Range") != want {
		t.Errorf("first range = %q, want %s", hdrs[0].Get("Range"), want)
	}
}

type countTransport struct {
	base  http.RoundTripper
	reads *atomic.Int64
}

func (c *countTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &countBody{ReadCloser: resp.Body, reads: c.reads}
	return resp, nil
}

type countBody struct {
	io.ReadCloser
	reads *atomic.Int64
}

func (b *countBody) Read(p []byte) (int, error) {
	b.reads.Add(1)
	return b.ReadCloser.Read(p)
}
