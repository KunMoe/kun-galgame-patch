package artifactclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var b2PutClient = &http.Client{}

const (
	putPartTimeout = 30 * time.Minute
	maxPutRetries  = 3
)

func ptr[T any](v T) *T { return &v }

// UploadFile puts a file on disk through init, a PUT straight to storage and
// complete, for jobs that upload without a browser.
func (c *Client) UploadFile(ctx context.Context, path, uploadName string, size int64, uploaderSub int) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	init, err := c.InitUpload(ctx, InitUploadRequest{
		Name:        uploadName,
		FileSize:    size,
		MimeType:    ptr(mimeForExt(uploadName)),
		Public:      ptr(true),
		UploaderSub: ptr(strconv.Itoa(uploaderSub)),
	})
	if err != nil {
		return "", 0, fmt.Errorf("init: %w", err)
	}

	var complete CompleteUploadRequest
	if init.Multipart {
		parts, err := putAllParts(ctx, f, size, init)
		if err != nil {
			return "", 0, err
		}
		complete.Parts = &parts
	} else {
		if init.UploadUrl == nil {
			return "", 0, fmt.Errorf("init returned no upload_url for single-part upload")
		}
		if _, err := putOne(ctx, *init.UploadUrl, f, 0, size); err != nil {
			return "", 0, fmt.Errorf("single PUT: %w", err)
		}
	}

	art2, err := c.CompleteUpload(ctx, init.Uuid, complete)
	if err != nil {
		return "", 0, fmt.Errorf("complete: %w", err)
	}
	return init.Uuid, art2.FileSize, nil
}

func putAllParts(ctx context.Context, f *os.File, size int64, init *InitUploadResponse) ([]CompletedPart, error) {
	if init.PartSize == nil || init.PartUrls == nil {
		return nil, fmt.Errorf("init multipart response missing part_size/part_urls")
	}
	partSize := *init.PartSize
	urls := *init.PartUrls
	parts := make([]CompletedPart, 0, len(urls))
	for _, p := range urls {
		start := int64(p.PartNumber-1) * partSize
		end := start + partSize
		if end > size {
			end = size
		}
		etag, err := putOne(ctx, p.Url, f, start, end-start)
		if err != nil {
			return nil, fmt.Errorf("part %d: %w", p.PartNumber, err)
		}
		parts = append(parts, CompletedPart{PartNumber: p.PartNumber, Etag: etag})
	}
	return parts, nil
}

func putOne(ctx context.Context, url string, f *os.File, start, length int64) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxPutRetries; attempt++ {
		etag, err := putOnce(ctx, url, io.NewSectionReader(f, start, length), length)
		if err == nil {
			return etag, nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	return "", lastErr
}

func putOnce(ctx context.Context, url string, body io.Reader, length int64) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, putPartTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodPut, url, body)
	if err != nil {
		return "", err
	}
	req.ContentLength = length
	resp, err := b2PutClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("PUT HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return strings.Trim(resp.Header.Get("ETag"), `"`), nil
}

func mimeForExt(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".zip":
		return "application/zip"
	case ".7z":
		return "application/x-7z-compressed"
	case ".rar":
		return "application/x-rar-compressed"
	default:
		return "application/octet-stream"
	}
}
