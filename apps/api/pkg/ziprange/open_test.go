package ziprange

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenStoredEntryInsideTheSuffix(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "work/", zip.Store, nil)
		writeEntry(t, w, "work/readme.txt", zip.Store, []byte("hello"))
	})
	log, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "work/readme.txt", 1<<20)
	if got := readAll(t, c); got != "hello" || c.Size != 5 || c.Truncated {
		t.Fatalf("content = %q size %d truncated %v", got, c.Size, c.Truncated)
	}
	hdrs := log.headers()
	assertListingRequests(t, hdrs)
	if len(hdrs) != 1 {
		t.Fatalf("requests = %d, want 1", len(hdrs))
	}
}

func TestOpenDeflatedEntry(t *testing.T) {
	text := strings.Repeat("[Config]\r\nWidth=1280\r\n", 400)
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "game/config.ini", zip.Deflate, []byte(text))
	})
	_, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "game/config.ini", 1<<20)
	if got := readAll(t, c); got != text || c.Size != int64(len(text)) {
		t.Fatalf("content length %d size %d, want %d", len(got), c.Size, len(text))
	}
}

func TestOpenStreamsAnEntryOutsideTheSuffix(t *testing.T) {
	big := noise(5 << 20)
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "work/cg.png", zip.Store, big)
		writeEntry(t, w, "work/readme.txt", zip.Store, []byte("hi"))
	})
	log, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "work/cg.png", 20<<20)
	got := readAll(t, c)
	if !bytes.Equal([]byte(got), big) {
		t.Fatalf("streamed %d bytes, want %d identical", len(got), len(big))
	}
	hdrs := log.headers()
	assertListingRequests(t, hdrs)
	if len(hdrs) != 3 {
		t.Fatalf("requests = %d, want suffix + local header + data", len(hdrs))
	}
	off := dataOffset(t, body, 0)
	want := "bytes=" + strconv.FormatInt(off, 10) + "-" + strconv.FormatInt(off+int64(len(big))-1, 10)
	if got := hdrs[len(hdrs)-1].Get("Range"); got != want {
		t.Fatalf("data range = %q, want %q", got, want)
	}
}

func TestOpenReadsASmallEntryWithItsHeaderInOneRequest(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "work/readme.txt", zip.Store, []byte("read me"))
		writeEntry(t, w, "work/cg.png", zip.Store, noise(3<<20))
	})
	log, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "work/readme.txt", 1<<20)
	if got := readAll(t, c); got != "read me" {
		t.Fatalf("content = %q", got)
	}
	hdrs := log.headers()
	if len(hdrs) != 2 {
		t.Fatalf("requests = %d, want suffix + one entry read", len(hdrs))
	}
	start, end, ok := testRange(hdrs[1].Get("Range"), int64(len(body)))
	if !ok || start != 0 || end-start+1 > 2<<10 {
		t.Fatalf("entry range = %q, want the local header and data only", hdrs[1].Get("Range"))
	}
}

func TestOpenReusesACachedDirectory(t *testing.T) {
	big := noise(5 << 20)
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "work/cg.png", zip.Store, big)
		writeEntry(t, w, "work/readme.txt", zip.Store, []byte("hi"))
	})
	log, srv := serveRange(t, body)
	o := NewOpener(srv.Client(), 64<<20)
	ctx := context.Background()
	first, err := o.Open(ctx, "uuid-1", srv.URL, "work/readme.txt", 1<<20)
	if err != nil || readAll(t, first) != "hi" {
		t.Fatalf("first open: %v", err)
	}
	before := len(log.headers())
	second, err := o.Open(ctx, "uuid-1", srv.URL, "work/cg.png", 20<<20)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	if got := readAll(t, second); len(got) != len(big) {
		t.Fatalf("second content = %d bytes", len(got))
	}
	for _, h := range log.headers()[before:] {
		if strings.HasPrefix(h.Get("Range"), "bytes=-") {
			t.Fatalf("second open re-read the directory: %q", h.Get("Range"))
		}
	}
}

func TestOpenTruncatesAStoredEntryWithoutReadingIt(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "huge.log", zip.Store, bytes.Repeat([]byte("x"), 9<<20))
		writeEntry(t, w, "end.txt", zip.Store, []byte("end"))
	})
	log, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "huge.log", 1<<20)
	if got := readAll(t, c); len(got) != 1<<20 || !c.Truncated {
		t.Fatalf("content = %d bytes truncated %v", len(got), c.Truncated)
	}
	var read int64
	for _, h := range log.headers() {
		start, end, _ := testRange(h.Get("Range"), int64(len(body)))
		read += end - start + 1
	}
	if read > 3<<20 {
		t.Fatalf("read %d bytes to show the first 1 MiB", read)
	}
}

func TestOpenTruncatesAtTheLimit(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "big.log", zip.Store, []byte(strings.Repeat("x", 100)))
	})
	_, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "big.log", 10)
	if got := readAll(t, c); got != strings.Repeat("x", 10) || c.Size != 10 || !c.Truncated {
		t.Fatalf("content = %q size %d truncated %v", got, c.Size, c.Truncated)
	}
}

func TestOpenRejectsACorruptEntry(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "a.txt", zip.Store, []byte("original"))
	})
	at := bytes.Index(body, []byte("original"))
	body[at] = 'O'
	_, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "a.txt", 1<<20)
	defer c.Close()
	if _, err := io.ReadAll(c); !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("read err = %v, want checksum", err)
	}
}

func TestOpenMissingEntryOrDirectory(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "work/", zip.Store, nil)
		writeEntry(t, w, "work/a.txt", zip.Store, []byte("a"))
	})
	_, srv := serveRange(t, body)
	for _, path := range []string{"work/", "work", "work/b.txt", ""} {
		if _, err := open(t, srv.Client(), srv.URL, path, 1<<20); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q) = %v, want not found", path, err)
		}
	}
}

func TestOpenFindsANormalizedName(t *testing.T) {
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, `./pack\docs\readme.txt`, zip.Store, []byte("ok"))
	})
	_, srv := serveRange(t, body)
	c := mustOpen(t, srv.Client(), srv.URL, "pack/docs/readme.txt", 1<<20)
	if got := readAll(t, c); got != "ok" {
		t.Fatalf("content = %q", got)
	}
}

func TestOpenRetriesAWholeObjectAnswerForTheData(t *testing.T) {
	big := noise(5 << 20)
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "cg.png", zip.Store, big)
	})
	_, ranged := serveRange(t, body)
	var log reqLog
	var served atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		if served.Add(1) == 3 {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(http.StatusOK)
			return
		}
		ranged.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c := mustOpen(t, srv.Client(), srv.URL, "cg.png", 20<<20)
	if got := readAll(t, c); len(got) != len(big) {
		t.Fatalf("streamed %d bytes, want %d", len(got), len(big))
	}
	if n := len(log.headers()); n != 4 {
		t.Fatalf("requests = %d, want 4", n)
	}
}

func TestOpenFailsWithoutReadingAWrongDataAnswer(t *testing.T) {
	big := noise(5 << 20)
	body := buildZip(t, func(w *zip.Writer) {
		writeEntry(t, w, "cg.png", zip.Store, big)
	})
	_, ranged := serveRange(t, body)
	var served atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if served.Add(1) >= 3 {
			w.Header().Set("Content-Range", "bytes 0-9/"+strconv.Itoa(len(body)))
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body[:10])
			return
		}
		ranged.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	if _, err := open(t, srv.Client(), srv.URL, "cg.png", 20<<20); err == nil {
		t.Fatal("expected a mismatched Content-Range to fail")
	}
}

func writeEntry(t *testing.T, w *zip.Writer, name string, method uint16, data []byte) {
	t.Helper()
	fw, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
}

func dataOffset(t *testing.T, body []byte, i int) int64 {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	off, err := zr.File[i].DataOffset()
	if err != nil {
		t.Fatal(err)
	}
	return off
}

func noise(n int) []byte {
	b := make([]byte, n)
	r := rand.NewChaCha8([32]byte{7})
	_, _ = r.Read(b)
	return b
}

func open(t *testing.T, hc *http.Client, rawURL, path string, limit int64) (*Content, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return NewOpener(hc, 64<<20).Open(ctx, rawURL, rawURL, path, limit)
}

func mustOpen(t *testing.T, hc *http.Client, rawURL, path string, limit int64) *Content {
	t.Helper()
	c, err := open(t, hc, rawURL, path, limit)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	return c
}

func readAll(t *testing.T, c *Content) string {
	t.Helper()
	defer c.Close()
	b, err := io.ReadAll(c)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}
