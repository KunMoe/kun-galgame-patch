package markdown_test

import (
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

func TestMentionLink(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// rel="nofollow" is appended by the bluemonday sanitize pass (UGCPolicy)
		// to every rendered <a>; the mention chip (class + data-id) and the
		// relative internal href both survive it.
		{
			name: "mention with /resource sub-route",
			in:   "hi [@kun](/user/1/resource) hello",
			want: `<a class="kun-mention" data-id="1" href="/user/1/resource" rel="nofollow">@kun</a>`,
		},
		{
			name: "bare /user/<userID>",
			in:   "[@yui](/user/42)",
			want: `<a class="kun-mention" data-id="42" href="/user/42" rel="nofollow">@yui</a>`,
		},
		{
			name: "non-mention link is unchanged",
			in:   "[click](https://example.com)",
			want: `<a href="https://example.com" rel="nofollow">click</a>`,
		},
		{
			name: "user link without leading @ falls back to plain link",
			in:   "[whatever](/user/7/resource)",
			want: `<a href="/user/7/resource" rel="nofollow">whatever</a>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := markdown.Render(tc.in)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected output to contain %q\n  got: %s", tc.want, out)
			}
		})
	}
}

func TestExtractMentionedUserIDs(t *testing.T) {
	in := "ping [@a](/user/1/resource), [@b](/user/2), [@a-again](/user/1) and [link](/about)"
	got := markdown.ExtractMentionedUserIDs(in)
	want := []int{1, 2}
	if len(got) != len(want) {
		t.Fatalf("userID count mismatch, got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("userID[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestRenderEmptyString(t *testing.T) {
	out, err := markdown.Render("")
	if err != nil {
		t.Fatalf("expected nil error for empty input: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

// TestRenderSanitizesRawHTML locks the server-side XSS boundary: goldmark runs
// with WithUnsafe (raw user HTML passes through) and the bluemonday allow-list
// (sanitize.go) neutralizes it. This is the only sanitizer now that the frontend
// binds *_html via v-html with no client-side DOMPurify — each `bad` substring
// (a live element, event handler, or unsafe scheme) must NOT survive. Note the
// property is "neutralized", not "escaped": a dangerous attribute is stripped
// while a safe carrier tag may remain (e.g. <img onerror> → <img src>).
func TestRenderSanitizesRawHTML(t *testing.T) {
	cases := []struct{ name, in, bad string }{
		{"script element", `<script>alert(1)</script>`, "<script"},
		{"style element", `<style>body{}</style>`, "<style"},
		{"img onerror handler", `<img src=x onerror="alert(1)">`, "onerror"},
		{"svg onload handler", `<svg onload="alert(1)"></svg>`, "<svg"},
		{"iframe element", `<iframe src="https://evil.example"></iframe>`, "<iframe"},
		{"anchor onclick handler", `<a href="/x" onclick="alert(1)">x</a>`, "onclick"},
		{"javascript: href", `<a href="javascript:alert(1)">x</a>`, "javascript:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := strings.ToLower(markdown.MustRender(tc.in))
			if strings.Contains(out, tc.bad) {
				t.Errorf("dangerous %q survived sanitization for input %q:\n  %s", tc.bad, tc.in, out)
			}
		})
	}
}

// TestSanitizePreservesRichContent is the companion to the XSS test: the
// allow-list must NOT strip anything this package's renderers legitimately emit,
// or real content silently breaks. Each `want` substring is a feature the policy
// has to let through (mention chip, image blur-up metadata, heading anchor, code
// language class, GFM task-list + table alignment, internal relative link).
func TestSanitizePreservesRichContent(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"mention chip class + data-id", "[@kun](/user/1)", `class="kun-mention" data-id="1"`},
		{"internal relative link href", "[patch](/patch/42)", `href="/patch/42"`},
		{"heading anchor id", "## 关于\n", `id="关于"`},
		{"fenced code language class", "```go\nx := 1\n```\n", `class="language-go"`},
		{"gfm task list checkbox", "- [x] done\n", `type="checkbox"`},
		{"gfm table cell alignment", "| a |\n|:-:|\n| b |\n", `align="center"`},
		{"strikethrough", "~~x~~", `<del>x</del>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := markdown.MustRender(tc.in)
			if !strings.Contains(out, tc.want) {
				t.Errorf("sanitizer stripped legitimate %q from input %q:\n  %s", tc.want, tc.in, out)
			}
		})
	}
}

// TestRenderStripsDangerousURLs locks goldmark's html.IsDangerousURL behavior
// for markdown links/images: javascript:/vbscript:/data: destinations must be
// dropped (href/src emitted empty), so a crafted [x](javascript:…) or
// ![x](javascript:…) can't execute when the HTML is bound with v-html.
func TestRenderStripsDangerousURLs(t *testing.T) {
	cases := []struct{ name, in, bad string }{
		{"javascript link", "[x](javascript:alert(1))", "javascript:"},
		{"javascript image", "![x](javascript:alert(1))", "javascript:"},
		{"vbscript link", "[x](vbscript:msgbox(1))", "vbscript:"},
		{"data text/html link", "[x](data:text/html;base64,PHN2Zz4=)", "data:text/html"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := strings.ToLower(markdown.MustRender(tc.in))
			if strings.Contains(out, tc.bad) {
				t.Errorf("dangerous URL %q leaked into output: %s", tc.bad, out)
			}
		})
	}
}

func TestRenderGFMStrikethrough(t *testing.T) {
	out := markdown.MustRender("~~done~~")
	if !strings.Contains(out, "<del>done</del>") {
		t.Errorf("expected <del>done</del>, got %s", out)
	}
}

func TestRenderWithTOCChineseHeadings(t *testing.T) {
	src := `# 关于我们

## 简介

### 鲲是什么

## 联系方式
`
	html, toc, err := markdown.RenderWithTOC(src)
	if err != nil {
		t.Fatalf("RenderWithTOC failed: %v", err)
	}

	if len(toc) != 4 {
		t.Fatalf("expected 4 TOC items, got %d: %#v", len(toc), toc)
	}
	want := []struct {
		id    string
		text  string
		level int
	}{
		{"关于我们", "关于我们", 1},
		{"简介", "简介", 2},
		{"鲲是什么", "鲲是什么", 3},
		{"联系方式", "联系方式", 2},
	}
	for i, w := range want {
		if toc[i].ID != w.id || toc[i].Text != w.text || toc[i].Level != w.level {
			t.Errorf("toc[%d] = %+v, want %+v", i, toc[i], w)
		}
	}

	// HTML should carry the same id values so anchor links work.
	for _, item := range toc {
		needle := `id="` + item.ID + `"`
		if !strings.Contains(html, needle) {
			t.Errorf("rendered HTML missing %s", needle)
		}
	}
}

func TestRenderWithTOCDuplicateHeadings(t *testing.T) {
	src := "## 资源\n\n## 资源\n\n## 资源\n"
	_, toc, err := markdown.RenderWithTOC(src)
	if err != nil {
		t.Fatalf("RenderWithTOC failed: %v", err)
	}
	got := []string{toc[0].ID, toc[1].ID, toc[2].ID}
	want := []string{"资源", "资源-1", "资源-2"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dedup id[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderWithTOCSkipsHeadingsBeyondLevel3(t *testing.T) {
	src := "# A\n\n## B\n\n### C\n\n#### D\n"
	_, toc, err := markdown.RenderWithTOC(src)
	if err != nil {
		t.Fatalf("RenderWithTOC failed: %v", err)
	}
	if len(toc) != 3 {
		t.Fatalf("expected only h1-h3 (3 items), got %d", len(toc))
	}
}

func TestContentImageTokenResolution(t *testing.T) {
	const hash = "278c8e45bb9622b74b6cccd200477aacb05c509c0b9632674eeb5972ab04acdf"

	// With a resolver wired, an exact /image/<hash> token is rewritten to the
	// resolved CDN URL; everything else (absolute URLs, malformed tokens) passes
	// through unchanged.
	markdown.SetContentImageResolver(func(h string) string {
		return "https://cdn.example.com/" + h[:2] + "/" + h[2:4] + "/" + h + ".webp"
	})
	t.Cleanup(func() { markdown.SetContentImageResolver(nil) })

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "content token is rewritten to CDN url",
			in:   "![pic](/image/" + hash + ")",
			want: `src="https://cdn.example.com/27/8c/` + hash + `.webp"`,
		},
		{
			name: "absolute legacy url is left untouched",
			in:   "![pic](https://image.moyu.moe/user_1/image/1-2.avif)",
			want: `src="https://image.moyu.moe/user_1/image/1-2.avif"`,
		},
		{
			name: "short/invalid hash is not treated as a token",
			in:   "![pic](/image/deadbeef)",
			want: `src="/image/deadbeef"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := markdown.MustRender(tc.in)
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected output to contain %q\n  got: %s", tc.want, out)
			}
		})
	}
}

func TestContentImageTokenWithoutResolverIsLeftAsIs(t *testing.T) {
	// No resolver wired (the default) → the token stays as-is, so the web
	// /image/:hash 302 route resolves it at request time.
	const hash = "278c8e45bb9622b74b6cccd200477aacb05c509c0b9632674eeb5972ab04acdf"
	out := markdown.MustRender("![pic](/image/" + hash + ")")
	if !strings.Contains(out, `src="/image/`+hash+`"`) {
		t.Errorf("expected token left untouched, got: %s", out)
	}
}
