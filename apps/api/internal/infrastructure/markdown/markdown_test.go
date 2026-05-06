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
