package ziprange

import (
	"archive/zip"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"net/http"
	"strings"
)

const (
	localHeaderBytes = 30
	localExtraSlack  = 1 << 10
	inlineBytes      = 4 << 20
)

var ErrNotFound = errors.New("ziprange: entry not found")

type Content struct {
	io.Reader
	io.Closer
	Size      int64
	Truncated bool
}

type Opener struct {
	hc    *http.Client
	cache *dirCache
}

func NewOpener(hc *http.Client, cacheBytes int64) *Opener {
	return &Opener{hc: hc, cache: newDirCache(cacheBytes)}
}

// The directory is cached under key, so key must name bytes that never change.
func (o *Opener) Open(ctx context.Context, key, rawURL, path string, limit int64) (*Content, error) {
	rr := newRangeReader(ctx, o.hc, rawURL)
	cached := o.cache.get(key)
	if cached != nil {
		rr.seed(cached)
	} else if err := rr.fetchSuffix(); err != nil {
		return nil, err
	}
	zr, err := parseDirectory(rr)
	if err != nil {
		return nil, err
	}
	if cached == nil {
		o.cache.put(key, rr.snapshot())
	}

	f := findFile(zr.File, path)
	if f == nil {
		return nil, ErrNotFound
	}
	if f.Method != zip.Store && f.Method != zip.Deflate {
		return nil, fmt.Errorf("%w: method %d", ErrUnsupported, f.Method)
	}
	size := int64(f.UncompressedSize64)
	truncated := size > limit
	want := int64(f.CompressedSize64)
	if truncated && f.Method == zip.Store {
		want = limit
	}

	rr.miss = localHeaderBytes + int64(len(f.Name)) + localExtraSlack
	if rr.miss+want <= inlineBytes {
		rr.miss += want
	}
	off, err := f.DataOffset()
	if err != nil {
		return nil, err
	}
	var raw io.ReadCloser
	if rr.covers(off, want) {
		raw = io.NopCloser(io.NewSectionReader(rr, off, want))
	} else if raw, err = rr.stream(off, want); err != nil {
		return nil, err
	}

	var body io.Reader = raw
	if f.Method == zip.Deflate {
		body = flate.NewReader(raw)
	}
	if truncated {
		return &Content{Reader: io.LimitReader(body, limit), Closer: raw, Size: limit, Truncated: true}, nil
	}
	checked := &checksumReader{r: body, hash: crc32.NewIEEE(), want: f.CRC32, left: size}
	return &Content{Reader: checked, Closer: raw, Size: size}, nil
}

func findFile(files []*zip.File, path string) *zip.File {
	if path == "" || strings.HasSuffix(path, "/") {
		return nil
	}
	for _, f := range files {
		if !f.FileInfo().IsDir() && normalizePath(f.Name) == path {
			return f
		}
	}
	return nil
}

type checksumReader struct {
	r    io.Reader
	hash hash.Hash32
	want uint32
	left int64
}

func (c *checksumReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.hash.Write(p[:n])
	c.left -= int64(n)
	if c.left == 0 {
		if c.hash.Sum32() != c.want {
			return n, zip.ErrChecksum
		}
		return n, io.EOF
	}
	if err == io.EOF {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
