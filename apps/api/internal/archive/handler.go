package archive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"time"

	"kun-galgame-patch-api/pkg/artifactclient"
	apperrors "kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"
	"kun-galgame-patch-api/pkg/ziprange"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

const (
	fileContentTimeout = 3 * time.Minute
	fileContentPolicy  = "default-src 'none'; style-src 'unsafe-inline'; sandbox"
)

var errArchiveRead = apperrors.New(50200, "读取文件失败，请稍后再试", http.StatusBadGateway)

type Handler struct {
	db     *gorm.DB
	art    *artifactclient.Client
	opener *ziprange.Opener
}

func NewHandler(db *gorm.DB, art *artifactclient.Client) *Handler {
	return &Handler{db: db, art: art, opener: ziprange.NewOpener(http.DefaultClient, 64<<20)}
}

func resourceID(c fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	return id, err == nil && id > 0
}

// visible applies the resource page's own gate: the work must be listable
// under the reader's content limit; listFiles and findFileSource add that the
// resource is public and hosted.
func (h *Handler) visible(c fiber.Ctx, id int) (bool, error) {
	var n int64
	q := h.db.WithContext(c.Context()).Table("patch").
		Where("patch.id = (SELECT galgame_id FROM patch_resource WHERE id = ?)", id)
	err := utils.ScopeListablePatches(q, utils.ContentLimitForListBrowse(c)).Count(&n).Error
	return n > 0, err
}

func (h *Handler) Files(c fiber.Ctx) error {
	id, ok := resourceID(c)
	if !ok {
		return response.Error(c, apperrors.ErrBadRequest("invalid resource id"))
	}
	if ok, err := h.visible(c, id); err != nil {
		slog.Error("archive files: visibility failed", "id", id, "error", err)
		return response.Error(c, apperrors.ErrInternal(""))
	} else if !ok {
		return response.Error(c, apperrors.ErrNotFound("resource not found"))
	}
	listing, err := listFiles(c.Context(), h.db, id)
	if err != nil {
		slog.Error("archive files failed", "id", id, "error", err)
		return response.Error(c, apperrors.ErrInternal(""))
	}
	if listing != nil && c.Query("full") != "1" {
		listing.Outline()
	}
	return response.OK(c, fiber.Map{"listing": listing, "preview": Kinds})
}

func (h *Handler) FileContent(c fiber.Ctx) error {
	id, ok := resourceID(c)
	name := c.Query("path")
	if !ok || name == "" {
		return response.Error(c, apperrors.ErrBadRequest("invalid file"))
	}
	kind, mediaType, ok := Lookup(name)
	if !ok {
		return response.Error(c, apperrors.ErrBadRequest("该类型的文件不支持在线查看"))
	}
	if ok, err := h.visible(c, id); err != nil {
		slog.Error("archive file: visibility failed", "id", id, "error", err)
		return response.Error(c, apperrors.ErrInternal(""))
	} else if !ok {
		return response.Error(c, apperrors.ErrNotFound("resource not found"))
	}
	src, err := findFileSource(c.Context(), h.db, id, name)
	if err != nil {
		slog.Error("archive file source failed", "id", id, "error", err)
		return response.Error(c, apperrors.ErrInternal(""))
	}
	if src == nil {
		return response.Error(c, apperrors.ErrNotFound("文件不存在"))
	}
	if src.Size > kind.MaxBytes && !kind.Truncate {
		return response.Error(c, apperrors.ErrBadRequest("文件太大，无法在线查看"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), fileContentTimeout)
	content, e := h.open(ctx, src, name, kind.MaxBytes)
	if e != nil {
		cancel()
		return response.Error(c, e)
	}
	c.Set(fiber.HeaderContentType, mediaType)
	c.Set(fiber.HeaderContentDisposition, inlineDisposition(name))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderContentSecurityPolicy, fileContentPolicy)
	c.Set(fiber.HeaderCacheControl, "private, max-age=3600")
	if kind.Key != "save" {
		return c.SendStream(&cancelOnClose{Content: content, cancel: cancel}, int(content.Size))
	}

	defer cancel()
	data, err := io.ReadAll(content)
	_ = content.Close()
	if err != nil {
		slog.Warn("archive file read failed", "path", name, "error", err)
		return response.Error(c, errArchiveRead)
	}
	text, err := decodeSave(name, data)
	if err != nil {
		return response.Error(c, apperrors.ErrBadRequest("无法解析这个存档"))
	}
	if len(text) > saveTextLimit {
		text = text[:saveTextLimit]
		c.Set("X-Content-Truncated", "true")
	}
	return c.SendStream(bytes.NewReader(text), len(text))
}

func (h *Handler) open(ctx context.Context, src *fileSource, name string, limit int64) (*ziprange.Content, *apperrors.AppError) {
	if h.art == nil || !h.art.Configured() {
		return nil, apperrors.New(50300, "站内文件服务暂不可用", http.StatusServiceUnavailable)
	}
	dl, err := h.art.Download(ctx, src.ArtifactUUID)
	if err != nil {
		slog.Warn("archive file download url failed", "uuid", src.ArtifactUUID, "error", err)
		return nil, errArchiveRead
	}
	content, err := h.opener.Open(ctx, src.ArtifactUUID, dl.Url, name, limit)
	switch {
	case err == nil:
		return content, nil
	case errors.Is(err, ziprange.ErrNotFound):
		return nil, apperrors.ErrNotFound("文件不存在")
	case errors.Is(err, ziprange.ErrUnsupported):
		return nil, apperrors.ErrBadRequest("该文件无法在线查看")
	default:
		slog.Warn("archive file read failed", "path", name, "error", err)
		return nil, errArchiveRead
	}
}

func inlineDisposition(name string) string {
	if v := mime.FormatMediaType("inline", map[string]string{"filename": path.Base(name)}); v != "" {
		return v
	}
	return "inline"
}

type cancelOnClose struct {
	*ziprange.Content
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.Content.Close()
	c.cancel()
	return err
}
