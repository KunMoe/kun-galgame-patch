package service

import (
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/pkg/communityclient"
)

// The TL0 sandbox holds a newcomer's first posts: created hidden, queued for
// review. The author still has to see their own, or posting looks like it
// silently failed and they post again.
func TestHeldPostIsVisibleOnlyToItsAuthor(t *testing.T) {
	cases := []struct {
		name     string
		status   int32
		authorID int64
		viewerID int
		want     bool
	}{
		{"held, author looking", communityclient.PostHeld, 3, 3, true},
		{"held, someone else", communityclient.PostHeld, 3, 4, false},
		{"held, anonymous", communityclient.PostHeld, 3, 0, false},
		{"visible to anyone", communityclient.PostVisible, 3, 0, true},
		// A tombstone keeps its post_number — it is what holds the wall's
		// numbering — so it renders as a stub rather than disappearing.
		{"tombstone still renders", communityclient.PostDeleted, 3, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := visibleTo(tc.status, tc.authorID, tc.viewerID); got != tc.want {
				t.Errorf("visibleTo = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildItemBlanksATombstone(t *testing.T) {
	post := communityclient.PostView{
		ID: 900, ThreadID: 7, AuthorID: 3,
		ContentRaw: "链接失效了", Status: communityclient.PostDeleted,
	}
	item := buildItem(post, PatchSurface(42), nil)
	if item.Content != "" || item.ContentHTML != "" {
		t.Errorf("tombstone still carries content: %q / %q", item.Content, item.ContentHTML)
	}
	if !item.Deleted {
		t.Error("deleted flag not set")
	}
}

func TestBuildItemKeepsReplyPointersNilWhenAbsent(t *testing.T) {
	root := buildItem(communityclient.PostView{ID: 900}, PatchSurface(42), nil)
	if root.ParentCommentID != nil || root.RootCommentID != nil {
		t.Errorf("a root post claims a parent: %+v / %+v", root.ParentCommentID, root.RootCommentID)
	}

	reply := buildItem(communityclient.PostView{ID: 901, ReplyToPostID: 900, RootPostID: 900},
		PatchSurface(42), nil)
	if reply.ParentCommentID == nil || *reply.ParentCommentID != 900 {
		t.Errorf("reply parent = %v", reply.ParentCommentID)
	}
}

func TestResourceSurfaceCarriesBothIDs(t *testing.T) {
	item := buildItem(communityclient.PostView{ID: 900}, ResourceSurface(678, 42), nil)
	if item.GalgameID != 42 {
		t.Errorf("galgame_id = %d, want 42", item.GalgameID)
	}
	if item.ResourceID == nil || *item.ResourceID != 678 {
		t.Errorf("resource_id = %v, want 678", item.ResourceID)
	}
	if got := buildItem(communityclient.PostView{ID: 900}, PatchSurface(42), nil).ResourceID; got != nil {
		t.Errorf("game wall claims resource_id %v", got)
	}
}

// Every permalink anchors on `#post-<id>`. `#comment-<n>` is the pre-cutover
// comment id and still resolves through the import's map, so the two shapes
// must never be the same — a post id and an old comment id can be equal.
func TestPermalinkAnchorsOnThePostID(t *testing.T) {
	game := permalink(anchor.Target{Link: "/galgame/42?tab=comment"}, 900)
	if game != "/galgame/42?tab=comment#post-900" {
		t.Errorf("game permalink = %q", game)
	}
	res := permalink(anchor.Target{Link: "/resource/678", ResourceID: 678}, 901)
	if res != "/resource/678#post-901" {
		t.Errorf("resource permalink = %q", res)
	}
	if strings.Contains(game, "#comment-") || strings.Contains(res, "#comment-") {
		t.Error("a permalink used the legacy anchor shape")
	}
}

// Without the window a long comment shows its opening and highlights nothing,
// because the match is a thousand characters further down.
func TestSnippetWindowsOnTheHit(t *testing.T) {
	body := strings.Repeat("前", 400) + "汉化补丁" + strings.Repeat("后", 400)
	got := snippet(body, "汉化")
	if !strings.Contains(got, "汉化") {
		t.Fatalf("snippet lost the hit: %q", got)
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("a windowed snippet should say it was cut: %q", got)
	}
	if n := len([]rune(got)); n > snippetLen+1 {
		t.Errorf("snippet is %d runes, cap is %d", n, snippetLen)
	}

	short := "短评论"
	if snippet(short, "短") != short {
		t.Error("a short comment was cut")
	}
}

// The like count and the viewer's own flag are read off the post the primitive
// answers with; this site keeps no mirror of either.
func TestBuildItemCarriesUpstreamReactions(t *testing.T) {
	item := buildItem(communityclient.PostView{
		ID: 900, ReactionCount: 4, ViewerReacted: true,
	}, PatchSurface(42), nil)
	if item.LikeCount != 4 || !item.IsLiked {
		t.Errorf("item = %+v", item)
	}
}
