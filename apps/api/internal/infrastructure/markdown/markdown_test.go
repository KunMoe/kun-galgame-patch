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
		{
			name: "mention with /resource sub-route",
			in:   "hi [@kun](/user/1/resource) hello",
			want: `<a class="kun-mention" data-uid="1" href="/user/1/resource">@kun</a>`,
		},
		{
			name: "bare /user/<uid>",
			in:   "[@yui](/user/42)",
			want: `<a class="kun-mention" data-uid="42" href="/user/42">@yui</a>`,
		},
		{
			name: "non-mention link is unchanged",
			in:   "[click](https://example.com)",
			want: `<a href="https://example.com">click</a>`,
		},
		{
			name: "user link without leading @ falls back to plain link",
			in:   "[whatever](/user/7/resource)",
			want: `<a href="/user/7/resource">whatever</a>`,
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

func TestExtractMentionedUIDs(t *testing.T) {
	in := "ping [@a](/user/1/resource), [@b](/user/2), [@a-again](/user/1) and [link](/about)"
	got := markdown.ExtractMentionedUIDs(in)
	want := []int{1, 2}
	if len(got) != len(want) {
		t.Fatalf("uid count mismatch, got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("uid[%d] = %d, want %d", i, got[i], want[i])
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

func TestRenderRawHTMLIsEscaped(t *testing.T) {
	out := markdown.MustRender("<script>alert(1)</script>")
	if strings.Contains(out, "<script>") {
		t.Errorf("raw <script> leaked into output: %s", out)
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
