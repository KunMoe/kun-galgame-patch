package ziprange

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Entry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type Listing struct {
	Entries    []Entry
	FileCount  int
	DirCount   int
	TotalBytes int64
}

var ErrUnsupported = errors.New("ziprange: unsupported archive")

func List(ctx context.Context, hc *http.Client, rawURL string) (Listing, error) {
	_, zr, err := readDirectory(ctx, hc, rawURL)
	if err != nil {
		return Listing{}, err
	}
	if len(zr.File) > maxListingEntries {
		return Listing{}, fmt.Errorf("%w: %d entries", ErrUnsupported, len(zr.File))
	}
	return listingFrom(zr.File), nil
}

func readDirectory(ctx context.Context, hc *http.Client, rawURL string) (*rangeReader, *zip.Reader, error) {
	rr := newRangeReader(ctx, hc, rawURL)
	if err := rr.fetchSuffix(); err != nil {
		return nil, nil, err
	}
	zr, err := parseDirectory(rr)
	return rr, zr, err
}

func parseDirectory(rr *rangeReader) (*zip.Reader, error) {
	zr, err := zip.NewReader(rr, rr.size)
	if errors.Is(err, zip.ErrFormat) {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, err
	}
	return zr, nil
}

func listingFrom(files []*zip.File) Listing {
	entries := make([]Entry, 0, len(files))
	dirs := make(map[string]struct{})
	var total int64
	var filesN int
	for _, f := range files {
		path := normalizePath(f.Name)
		if path == "" {
			continue
		}
		if f.FileInfo().IsDir() && !strings.HasSuffix(path, "/") {
			path += "/"
		}
		if strings.HasSuffix(path, "/") {
			entries = append(entries, Entry{Path: path})
			addDirs(dirs, path)
			continue
		}
		n := int64(f.UncompressedSize64)
		total += n
		filesN++
		entries = append(entries, Entry{Path: path, Size: n})
		addDirs(dirs, path)
	}
	return Listing{Entries: entries, FileCount: filesN, DirCount: len(dirs), TotalBytes: total}
}

func addDirs(dirs map[string]struct{}, path string) {
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			dirs[path[:i+1]] = struct{}{}
		}
	}
}

func normalizePath(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	for {
		switch {
		case strings.HasPrefix(name, "/"):
			name = name[1:]
		case strings.HasPrefix(name, "./"):
			name = name[2:]
		default:
			return strings.ToValidUTF8(name, "\uFFFD")
		}
	}
}
