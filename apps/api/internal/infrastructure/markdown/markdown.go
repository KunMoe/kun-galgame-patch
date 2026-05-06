// Package markdown provides markdown-to-sanitized-HTML rendering.
//
// Used for user-editable rich text such as patch comments, resource notes and
// the Wiki-sourced introduction shown on the patch detail page.
//
// Built on goldmark + GFM extensions. A small custom HTML renderer detects
// "@mention"-style links — markdown of the form `[@username](/user/<id>/...)`
// — and emits them with a `kun-mention` class plus a `data-uid` attribute so
// the frontend can style and behave them differently from regular links.
package markdown

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// mentionURLRegex matches the destination of a mention link: `/user/<digits>`
// optionally followed by a sub-route. The captured uid is surfaced via a
// `data-uid` attribute on the rendered <a>.
var mentionURLRegex = regexp.MustCompile(`^/user/(\d+)(?:/.*)?$`)

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
		_, _ = w.WriteString(`<a class="kun-mention" data-uid="`)
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
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
		renderer.WithNodeRenderers(
			util.Prioritized(newMentionLinkRenderer(), 99),
		),
		// We deliberately do NOT enable html.WithUnsafe — raw HTML inside user
		// content is dropped, matching the legacy markdownToHtml behavior.
	),
)

// Render renders markdown text to HTML.
func Render(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
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

// ExtractMentionedUIDs scans markdown source for [@text](/user/<id>/...)
// patterns and returns the unique uids in order of first occurrence.
func ExtractMentionedUIDs(src string) []int {
	matches := mentionPatternRegex.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(matches))
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		uid, err := strconv.Atoi(m[1])
		if err != nil || uid <= 0 {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}
