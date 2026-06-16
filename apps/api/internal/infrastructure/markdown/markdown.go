// Package markdown provides markdown-to-sanitized-HTML rendering.
//
// Used for user-editable rich text such as patch comments, resource notes and
// the Wiki-sourced introduction shown on the patch detail page.
//
// Built on goldmark + GFM extensions. A small custom HTML renderer detects
// "@mention"-style links — markdown of the form `[@username](/user/<id>/...)`
// — and emits them with a `kun-mention` class plus a `data-id` attribute so
// the frontend can style and behave them differently from regular links.
package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// TOCItem is a single heading entry surfaced to the frontend table of contents.
type TOCItem struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Level int    `json:"level"`
}

// cjkIDs is goldmark's parser.IDs implementation but rewritten to preserve CJK
// (and any other non-ASCII letter) characters in heading IDs. The default
// implementation in goldmark/parser/parser.go silently skips multi-byte runes,
// so a heading like "## 关于我们" ends up with an empty slug and falls back to
// "heading-1" — making the URL fragments useless and unstable across the
// document.
type cjkIDs struct {
	values map[string]bool
}

func newCJKIDs() parser.IDs {
	return &cjkIDs{values: map[string]bool{}}
}

func (s *cjkIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	raw := strings.ToLower(strings.TrimSpace(string(value)))
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		switch {
		case unicode.IsSpace(r):
			b.WriteByte('-')
		case r == '-' || r == '_':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = "section"
	}
	if !s.values[id] {
		s.values[id] = true
		return []byte(id)
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d", id, i)
		if !s.values[candidate] {
			s.values[candidate] = true
			return []byte(candidate)
		}
	}
}

func (s *cjkIDs) Put(value []byte) { s.values[string(value)] = true }

// mentionURLRegex matches the destination of a mention link: `/user/<digits>`
// optionally followed by a sub-route. The captured userID is surfaced via a
// `data-id` attribute on the rendered <a>.
var mentionURLRegex = regexp.MustCompile(`^/user/(\d+)(?:/.*)?$`)

// contentImageRefRegex matches a domain-agnostic content image token,
// `/image/<64-hex-hash>`, that user markdown stores instead of an absolute CDN
// URL (image_service 契约 04 §"内容内嵌图的域名无关引用"). The hash is resolved
// to a real CDN URL at render time via resolveContentImage.
var contentImageRefRegex = regexp.MustCompile(`^/image/([0-9a-f]{64})$`)

// resolveContentImage maps a content image token's hash to a fully-qualified
// image_service CDN URL. Wired once at startup (app.go) to imageclient.MainURL.
// When nil (tests, or image_service unconfigured) the token is left untouched —
// the web `/image/:hash` 302 route then resolves it at request time (the
// contract's fallback path).
var resolveContentImage func(hash string) string

// SetContentImageResolver wires the hash→CDN-URL resolver used by the content
// image AST transformer. Call once at startup, before serving.
func SetContentImageResolver(fn func(hash string) string) { resolveContentImage = fn }

// contentImageTransformer rewrites `![](...)` image destinations of the form
// `/image/<hash>` into the resolved CDN URL during parsing, so server-rendered
// content embeds direct CDN URLs (the contract's "fast path"; the 302 route is
// the fallback for raw-markdown / editor-preview consumers). Anything that
// isn't an exact content-image token passes through unchanged, so absolute URLs
// (incl. not-yet-migrated legacy ones) and other links are unaffected.
type contentImageTransformer struct{}

func (contentImageTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	resolve := resolveContentImage
	if resolve == nil {
		return
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
		if url := resolve(string(m[1])); url != "" {
			img.Destination = []byte(url)
		}
		return ast.WalkContinue, nil
	})
}

// mentionPatternRegex pulls uids straight from markdown source for callers
// that need to know "who got mentioned" without rendering — e.g. the comment
// service sending notifications.
var mentionPatternRegex = regexp.MustCompile(`\[@[^\]]*\]\(/user/(\d+)(?:/[^)]*)?\)`)

// mentionLinkRenderer overrides the default link renderer for mention-style
// links and falls back to the standard renderer otherwise.
type mentionLinkRenderer struct {
	html.Config
}

func newMentionLinkRenderer() renderer.NodeRenderer {
	return &mentionLinkRenderer{Config: html.NewConfig()}
}

func (r *mentionLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

// linkText extracts the rendered text of a link's children, just enough to
// answer "does this link's label start with @?". We only look at *ast.Text
// nodes — emphasis/strong wrappers around an @ are uncommon and not worth
// handling specially.
func linkText(source []byte, n *ast.Link) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		}
	}
	return b.String()
}

func (r *mentionLinkRenderer) renderLink(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	link := n.(*ast.Link)
	dest := string(link.Destination)

	uidMatch := mentionURLRegex.FindStringSubmatch(dest)
	isMention := false
	if uidMatch != nil && strings.HasPrefix(linkText(source, link), "@") {
		isMention = true
	}

	if !isMention {
		return defaultLinkRender(w, link, entering, &r.Config)
	}

	if entering {
		_, _ = w.WriteString(`<a class="kun-mention" data-id="`)
		_, _ = w.WriteString(uidMatch[1])
		_, _ = w.WriteString(`" href="`)
		_, _ = w.Write(util.EscapeHTML(util.URLEscape([]byte(dest), true)))
		_, _ = w.WriteString(`">`)
	} else {
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}

// defaultLinkRender mirrors goldmark/renderer/html.(*Renderer).renderLink so
// non-mention links continue to render exactly as before. Reproduced inline
// because the upstream method is unexported.
func defaultLinkRender(w util.BufWriter, link *ast.Link, entering bool, cfg *html.Config) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<a href="`)
		if cfg.Unsafe || !html.IsDangerousURL(link.Destination) {
			_, _ = w.Write(util.EscapeHTML(util.URLEscape(link.Destination, true)))
		}
		_ = w.WriteByte('"')
		if link.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(util.EscapeHTML(link.Title))
			_ = w.WriteByte('"')
		}
		if link.Attributes() != nil {
			html.RenderAttributes(w, link, html.LinkAttributeFilter)
		}
		_ = w.WriteByte('>')
	} else {
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}

// md is the configured singleton. The mention renderer registers at priority
// 99 (lower than the default link renderer's 200) so it wins for ast.KindLink.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Linkify,
		extension.Strikethrough,
		extension.Table,
		extension.TaskList,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		// Resolve domain-agnostic content image tokens (/image/<hash>) to CDN
		// URLs during parse, so both Render and RenderWithTOC (which share this
		// parser) emit direct image src. Priority is arbitrary — it only walks
		// images, independent of other transformers.
		parser.WithASTTransformers(
			util.Prioritized(contentImageTransformer{}, 100),
		),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
		renderer.WithNodeRenderers(
			util.Prioritized(newMentionLinkRenderer(), 99),
		),
		// SECURITY BOUNDARY — do NOT enable html.WithUnsafe.
		//
		// This is the ONLY XSS sanitization for user content now: the web
		// frontend renders these *_html fields via v-html with no client-side
		// sanitizer (the old DOMPurify-on-jsdom was removed — it leaked SSR
		// memory and broke the Nitro build). With WithUnsafe off, goldmark
		// escapes raw user HTML (<script>, <img onerror>, …) and the default
		// link/image renderers run html.IsDangerousURL, which drops
		// javascript:/vbscript:/data: URLs (see defaultLinkRender above and the
		// XSS tests in markdown_test.go). If you ever need raw-HTML passthrough,
		// you MUST add a server-side allow-list sanitizer (e.g. bluemonday)
		// here first — enabling WithUnsafe alone reopens stored XSS.
	),
)

// Render renders markdown text to HTML.
func Render(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	var buf bytes.Buffer
	// Each call gets its own parser context so heading IDs do not leak across
	// documents (the default ids registry is shared otherwise).
	ctx := parser.NewContext(parser.WithIDs(newCJKIDs()))
	if err := md.Convert([]byte(src), &buf, parser.WithContext(ctx)); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MustRender returns the original text on render failure (as a fallback).
func MustRender(src string) string {
	out, err := Render(src)
	if err != nil {
		return src
	}
	return out
}

// RenderWithTOC renders markdown to HTML and additionally returns a flat list
// of headings (h1-h3) suitable for a "本页索引" sidebar. IDs come from the same
// CJK-friendly slugifier used by the renderer, so anchor links match heading
// `id` attributes exactly.
func RenderWithTOC(src string) (string, []TOCItem, error) {
	if src == "" {
		return "", nil, nil
	}

	source := []byte(src)
	ids := newCJKIDs()
	ctx := parser.NewContext(parser.WithIDs(ids))

	// Parse first so we can walk the AST for headings while sharing the same
	// ids registry — this guarantees the TOC ids match what the renderer
	// emits as `id="..."` on each <h*>.
	doc := md.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))

	toc := make([]TOCItem, 0, 16)
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok || h.Level > 3 {
			return ast.WalkContinue, nil
		}
		idAttr, _ := h.AttributeString("id")
		var id string
		switch v := idAttr.(type) {
		case []byte:
			id = string(v)
		case string:
			id = v
		}
		if id == "" {
			return ast.WalkContinue, nil
		}
		toc = append(toc, TOCItem{
			ID:    id,
			Text:  string(h.Text(source)),
			Level: h.Level,
		})
		return ast.WalkContinue, nil
	})

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, source, doc); err != nil {
		return "", nil, err
	}
	return buf.String(), toc, nil
}

// ExtractMentionedUserIDs scans markdown source for [@text](/user/<id>/...)
// patterns and returns the unique uids in order of first occurrence.
func ExtractMentionedUserIDs(src string) []int {
	matches := mentionPatternRegex.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(matches))
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		userID, err := strconv.Atoi(m[1])
		if err != nil || userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		out = append(out, userID)
	}
	return out
}
