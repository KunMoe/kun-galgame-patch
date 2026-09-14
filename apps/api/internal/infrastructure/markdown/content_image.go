package markdown

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// A content image is persisted as `/image/<hash>[_variant]` and resolved to a
// CDN URL at render time. The token names the bytes; a URL names a host, and a
// host outlives nothing. The sticker packs handed consumers absolute URLs and
// the forum stored them verbatim -- 215 stickers across 1,620 posts went to
// broken images the day the old sticker host stopped serving static files, and
// the URLs that replaced them weld today's CDN domain into every post that uses
// one.
//
// The variant suffix is load-bearing rather than decorative: a sticker is
// `_320` (what a picker grid renders) and a bare hash is the full-size
// original. Reading the token without it resolved every sticker to the original
// and, worse, refused to match at all in the anchored form.
var contentImageRefRegex = regexp.MustCompile(`^/image/([0-9a-f]{64})(?:_([a-z0-9]+))?$`)

var contentImageTokenRegex = regexp.MustCompile(`/image/([0-9a-f]{64})(?:_([a-z0-9]+))?`)

var resolveContentImage func(hash, variant string) string

func SetContentImageResolver(fn func(hash, variant string) string) { resolveContentImage = fn }

func init() {
	html.ImageAttributeFilter = html.ImageAttributeFilter.Extend([]byte("data-thumbhash"))
}

type ImageMeta struct {
	Width     int
	Height    int
	Thumbhash string
}

var resolveContentImageMeta func(hashes []string) map[string]ImageMeta

func SetContentImageMetaResolver(fn func(hashes []string) map[string]ImageMeta) {
	resolveContentImageMeta = fn
}

var imageMetaContextKey = parser.NewContextKey()

// Bare tokens only. The meta face answers for the original object, so pinning
// its width and height on a `_320` sticker lays a 320px image out at full size.
func contentImageHashes(src string) []string {
	ms := contentImageTokenRegex.FindAllStringSubmatch(src, -1)
	if len(ms) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ms))
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		if m[2] != "" {
			continue
		}
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		out = append(out, m[1])
	}
	return out
}

func newRenderContext(src string) parser.Context {
	ctx := parser.NewContext(parser.WithIDs(newCJKIDs()))
	if resolveContentImageMeta != nil {
		if hashes := contentImageHashes(src); len(hashes) > 0 {
			if meta := resolveContentImageMeta(hashes); len(meta) > 0 {
				ctx.Set(imageMetaContextKey, meta)
			}
		}
	}
	return ctx
}

func ResolveContentImageTokens(src string) string {
	resolve := resolveContentImage
	if resolve == nil || src == "" {
		return src
	}
	return contentImageTokenRegex.ReplaceAllStringFunc(src, func(tok string) string {
		m := contentImageTokenRegex.FindStringSubmatch(tok)
		if url := resolve(m[1], m[2]); url != "" {
			return url
		}
		return tok
	})
}

type contentImageTransformer struct{}

func (contentImageTransformer) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	resolve := resolveContentImage
	if resolve == nil {
		return
	}
	var meta map[string]ImageMeta
	if v := pc.Get(imageMetaContextKey); v != nil {
		meta, _ = v.(map[string]ImageMeta)
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		img, ok := n.(*ast.Image)
		if !ok {
			return ast.WalkContinue, nil
		}
		m := contentImageRefRegex.FindSubmatch(img.Destination)
		if m == nil {
			return ast.WalkContinue, nil
		}
		hash, variant := string(m[1]), string(m[2])
		if url := resolve(hash, variant); url != "" {
			img.Destination = []byte(url)
		}
		if variant != "" {
			return ast.WalkContinue, nil
		}
		if im, ok := meta[hash]; ok {
			if im.Width > 0 {
				img.SetAttributeString("width", []byte(strconv.Itoa(im.Width)))
			}
			if im.Height > 0 {
				img.SetAttributeString("height", []byte(strconv.Itoa(im.Height)))
			}
			if im.Thumbhash != "" {
				img.SetAttributeString("data-thumbhash", []byte(im.Thumbhash))
			}
		}
		return ast.WalkContinue, nil
	})
}

// The hosts image_service has served content-addressed objects from. Folding
// one of these URLs into a token is lossless because the token resolves back
// through the same store; folding an arbitrary host is not, since
// `/aa/bb/<hash>.webp` somewhere else is not promised to be the same bytes.
var contentImageHosts = map[string]bool{
	"image.kungal.iloveren.link": true,
	"image.kungal.com":           true,
}

// RegisterContentImageHost adds the CDN this deployment is configured with, so
// a domain move does not need this file edited before content written under the
// new host stops welding it in.
func RegisterContentImageHost(base string) {
	base = strings.TrimSpace(base)
	if base == "" {
		return
	}
	base = strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
	if host, _, ok := strings.Cut(base, "/"); ok {
		base = host
	}
	if base != "" {
		contentImageHosts[base] = true
	}
}

var contentImageURLRegex = regexp.MustCompile(
	`https?://([A-Za-z0-9.-]+)/([0-9a-f]{2})/([0-9a-f]{2})/([0-9a-f]{64})(?:_([a-z0-9]+))?\.webp`)

// NormalizeContentImageURLs folds an absolute image_service URL back into the
// token before the text is stored. Without it the store-a-token rule only holds
// for what this site's own uploader produced: the sticker picker hands the
// editor a CDN URL, and anything pasted from a browser is one too. Making it a
// write-time rule is what keeps a future CDN move a one-line config change
// rather than another backfill.
func NormalizeContentImageURLs(src string) string {
	if src == "" || !strings.Contains(src, "//") {
		return src
	}
	return contentImageURLRegex.ReplaceAllStringFunc(src, func(u string) string {
		m := contentImageURLRegex.FindStringSubmatch(u)
		host, hash, variant := m[1], m[4], m[5]
		if !contentImageHosts[host] || m[2] != hash[:2] || m[3] != hash[2:4] {
			return u
		}
		if variant != "" {
			return "/image/" + hash + "_" + variant
		}
		return "/image/" + hash
	})
}
