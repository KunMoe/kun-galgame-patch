package markdown_test

import (
	"testing"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

func TestNormalizeContentImageURLs(t *testing.T) {
	const hash = "278c8e45bb9622b74b6cccd200477aacb05c509c0b9632674eeb5972ab04acdf"

	markdown.RegisterContentImageHost("https://cdn.example.com/")
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "sticker url folds to a variant token",
			in:   "![](https://image.kungal.iloveren.link/27/8c/" + hash + "_320.webp)",
			want: "![](/image/" + hash + "_320)",
		},
		{
			name: "the configured CDN folds too",
			in:   "![](https://cdn.example.com/27/8c/" + hash + ".webp)",
			want: "![](/image/" + hash + ")",
		},
		{
			name: "an unknown host is left alone",
			in:   "![](https://example.org/27/8c/" + hash + ".webp)",
			want: "![](https://example.org/27/8c/" + hash + ".webp)",
		},
		{
			// The shard prefix is derived from the hash, so a URL where the two
			// disagree is not an object this store ever addressed.
			name: "a shard prefix that is not the hash prefix is left alone",
			in:   "![](https://image.kungal.iloveren.link/aa/bb/" + hash + ".webp)",
			want: "![](https://image.kungal.iloveren.link/aa/bb/" + hash + ".webp)",
		},
		{
			name: "a token is already normal",
			in:   "![](/image/" + hash + "_320)",
			want: "![](/image/" + hash + "_320)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := markdown.NormalizeContentImageURLs(tc.in); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}
