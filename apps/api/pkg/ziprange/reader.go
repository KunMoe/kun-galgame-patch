package ziprange

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

const (
	chunkBytes          = 1 << 20
	userAgent           = "letmoe-artifact-listing"
	wholeObjectAttempts = 3
)

var (
	maxDirectoryBytes int64 = 16 << 20
	maxListingEntries       = 50_000
)

type segment struct {
	off  int64
	data []byte
}

type rangeReader struct {
	ctx     context.Context
	hc      *http.Client
	url     string
	size    int64
	segs    []segment
	fetched int64
	miss    int64
}

func newRangeReader(ctx context.Context, hc *http.Client, rawURL string) *rangeReader {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &rangeReader{ctx: ctx, hc: hc, url: rawURL, size: -1}
}

func (r *rangeReader) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := 0
	for n < len(p) {
		o := off + int64(n)
		if r.size >= 0 && o >= r.size {
			return n, io.EOF
		}
		hit := r.hit(o)
		if hit == nil {
			if err := r.fetchAt(o); err != nil {
				return n, err
			}
			if r.hit(o) == nil {
				return n, fmt.Errorf("ziprange: range at %d did not cover the offset", o)
			}
			continue
		}
		n += copy(p[n:], hit)
	}
	return n, nil
}

func (r *rangeReader) hit(o int64) []byte {
	for _, s := range r.segs {
		if o >= s.off && o < s.off+int64(len(s.data)) {
			return s.data[o-s.off:]
		}
	}
	return nil
}

func (r *rangeReader) fetchSuffix() error {
	return r.fetch("bytes=-" + strconv.Itoa(chunkBytes))
}

func (r *rangeReader) fetchAt(o int64) error {
	n := r.miss
	if n <= 0 {
		n = chunkBytes
	}
	end := o + n
	if r.size >= 0 && end > r.size {
		end = r.size
	}
	if end <= o {
		return io.EOF
	}
	return r.fetch("bytes=" + strconv.FormatInt(o, 10) + "-" + strconv.FormatInt(end-1, 10))
}

func (r *rangeReader) ranged(rng string) (*http.Response, error) {
	resp, err := r.get(rng)
	// 2026-10-13: Cloudflare answered the first ranged request for an object it had
	// not served yet with 200 BYPASS and the whole object (5 of 10 fresh objects in
	// production); the request right after it was a 206.
	for attempt := 1; err == nil && wholeObject(resp) && attempt < wholeObjectAttempts; attempt++ {
		resp.Body.Close()
		resp, err = r.get(rng)
	}
	return resp, err
}

func (r *rangeReader) fetch(rng string) error {
	resp, err := r.ranged(rng)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		if resp.StatusCode == http.StatusOK && !wholeObject(resp) {
			return r.takeWhole(resp)
		}
		return fmt.Errorf("ziprange: range %s: status %d", rng, resp.StatusCode)
	}
	start, end, total, err := parseContentRange(resp.Header.Get("Content-Range"))
	if err != nil {
		return err
	}
	need := end - start + 1
	if err := r.admit(need); err != nil {
		return err
	}
	buf := make([]byte, need)
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		return err
	}
	r.fetched += need
	r.segs = append(r.segs, segment{off: start, data: buf})
	if r.size < 0 {
		r.size = total
		return nil
	}
	if r.size != total {
		return fmt.Errorf("ziprange: content-range total changed from %d to %d", r.size, total)
	}
	return nil
}

func (r *rangeReader) snapshot() *directory {
	var n int64
	for _, s := range r.segs {
		n += int64(len(s.data))
	}
	return &directory{size: r.size, segs: slices.Clone(r.segs), bytes: n}
}

func (r *rangeReader) seed(d *directory) {
	r.size = d.size
	r.segs = slices.Clone(d.segs)
}

func (r *rangeReader) covers(off, n int64) bool {
	for end := off + n; off < end; {
		hit := r.hit(off)
		if hit == nil {
			return false
		}
		off += int64(len(hit))
	}
	return true
}

func (r *rangeReader) stream(off, n int64) (io.ReadCloser, error) {
	last := off + n - 1
	rng := "bytes=" + strconv.FormatInt(off, 10) + "-" + strconv.FormatInt(last, 10)
	resp, err := r.ranged(rng)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("ziprange: range %s: status %d", rng, resp.StatusCode)
	}
	start, end, total, err := parseContentRange(resp.Header.Get("Content-Range"))
	if err == nil && (start != off || end != last || total != r.size) {
		err = fmt.Errorf("ziprange: range %s answered %q", rng, resp.Header.Get("Content-Range"))
	}
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

func (r *rangeReader) get(rng string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", rng)
	// 2026-10-13: net/http's default gzip makes Cloudflare drop Range and return the whole object.
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", userAgent)
	return r.hc.Do(req)
}

func wholeObject(resp *http.Response) bool {
	return resp.StatusCode == http.StatusOK &&
		(resp.ContentLength < 0 || resp.ContentLength > int64(chunkBytes))
}

func (r *rangeReader) takeWhole(resp *http.Response) error {
	n := resp.ContentLength
	if err := r.admit(n); err != nil {
		return err
	}
	if r.size >= 0 && n != r.size {
		return fmt.Errorf("ziprange: whole body %d does not match known size %d", n, r.size)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		return err
	}
	r.fetched += n
	r.segs = append(r.segs, segment{off: 0, data: buf})
	r.size = n
	return nil
}

func (r *rangeReader) admit(n int64) error {
	if n < 0 || n > maxDirectoryBytes || r.fetched > maxDirectoryBytes-n {
		return fmt.Errorf("%w: central directory fetch would exceed %d bytes", ErrUnsupported, maxDirectoryBytes)
	}
	return nil
}

func parseContentRange(v string) (start, end, total int64, err error) {
	v = strings.TrimSpace(v)
	rest, ok := strings.CutPrefix(v, "bytes ")
	if !ok {
		return 0, 0, 0, fmt.Errorf("ziprange: content-range %q", v)
	}
	rangePart, totalPart, ok := strings.Cut(rest, "/")
	if !ok || totalPart == "*" || strings.Contains(rangePart, "*") {
		return 0, 0, 0, fmt.Errorf("ziprange: content-range %q", v)
	}
	a, b, ok := strings.Cut(rangePart, "-")
	if !ok {
		return 0, 0, 0, fmt.Errorf("ziprange: content-range %q", v)
	}
	start, err = strconv.ParseInt(strings.TrimSpace(a), 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ziprange: content-range %q", v)
	}
	end, err = strconv.ParseInt(strings.TrimSpace(b), 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ziprange: content-range %q", v)
	}
	total, err = strconv.ParseInt(strings.TrimSpace(totalPart), 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ziprange: content-range %q", v)
	}
	if start < 0 || end < start || end >= total {
		return 0, 0, 0, fmt.Errorf("ziprange: content-range %q", v)
	}
	return start, end, total, nil
}
