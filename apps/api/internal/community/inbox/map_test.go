package inbox

import (
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/pkg/communityclient"
)

func TestMessageFor(t *testing.T) {
	postID := int64(900)
	game := anchor.Target{Link: "/galgame/42?tab=comment", Title: "クラナド", PatchID: 42}
	resource := anchor.Target{Link: "/resource/678", Title: "クラナド", PatchID: 42, ResourceID: 678}
	untitled := anchor.Target{Link: "/galgame/42?tab=comment", PatchID: 42}

	cases := []struct {
		name    string
		n       communityclient.NotificationView
		wall    anchor.Target
		excerpt string
		want    mappedRow
	}{
		{
			name: "replied",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindReplied, PostID: &postID},
			wall: game,
			want: mappedRow{Type: "comment", Content: "回复了您的评论", Link: "/galgame/42?tab=comment#post-900"},
		},
		{
			name: "no post id keeps the wall link",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindReplied},
			wall: game,
			want: mappedRow{Type: "comment", Content: "回复了您的评论", Link: "/galgame/42?tab=comment"},
		},
		{
			name:    "mentioned carries the excerpt",
			n:       communityclient.NotificationView{Kind: communityclient.NotificationKindMentioned, PostID: &postID},
			wall:    resource,
			excerpt: "你好 [@x](/user/3)",
			want:    mappedRow{Type: "mention", Content: "你好 [@x](/user/3)", Link: "/resource/678#post-900"},
		},
		{
			name:    "mentioned excerpt is cut by runes",
			n:       communityclient.NotificationView{Kind: communityclient.NotificationKindMentioned},
			wall:    game,
			excerpt: strings.Repeat("汉", 300),
			want:    mappedRow{Type: "mention", Content: strings.Repeat("汉", 233), Link: game.Link},
		},
		{
			name: "mentioned without a readable post",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindMentioned},
			wall: game,
			want: mappedRow{Type: "mention", Content: "在评论中提到了您", Link: game.Link},
		},
		{
			name: "posted on a game wall",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindPosted, ItemCount: 3, ActorCount: 1},
			wall: game,
			want: mappedRow{Type: "commentWatch", Content: "「クラナド」的评论区有 3 条新评论", Link: game.Link},
		},
		{
			name: "posted on a resource wall",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindPosted, ItemCount: 2, ActorCount: 1},
			wall: resource,
			want: mappedRow{Type: "commentWatch", Content: "「クラナド」的补丁资源评论区有 2 条新评论", Link: resource.Link},
		},
		{
			name: "posted without a title",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindPosted, ItemCount: 4},
			wall: untitled,
			want: mappedRow{Type: "commentWatch", Content: "您关注的评论区有 4 条新评论", Link: untitled.Link},
		},
		{
			name: "posted fold names its authors",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindPosted, ItemCount: 5, ActorCount: 3},
			wall: game,
			want: mappedRow{Type: "commentWatch", Content: "「クラナド」的评论区有 5 条新评论（3 人）", Link: game.Link},
		},
		{
			name: "liked",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindLiked, ActorCount: 1},
			wall: game,
			want: mappedRow{Type: "likeComment", Content: "赞了您的评论", Link: game.Link},
		},
		{
			name: "liked fold",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindLiked, ActorCount: 4},
			wall: game,
			want: mappedRow{Type: "likeComment", Content: "等 4 人赞了您的评论", Link: game.Link},
		},
		{
			name: "read upstream arrives read",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindReplied, ReadAt: "2026-09-16T00:00:00Z"},
			wall: game,
			want: mappedRow{Type: "comment", Content: "回复了您的评论", Status: 1, Link: game.Link},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !mirrored(tc.n.Kind) {
				t.Fatalf("kind %d is not mirrored", tc.n.Kind)
			}
			if got := messageFor(tc.n, tc.wall, tc.excerpt); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestFollowMessage(t *testing.T) {
	actor := int64(5)
	cases := []struct {
		name string
		n    communityclient.NotificationView
		want mappedRow
	}{
		{
			name: "single",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindFollowed, ActorID: &actor, ActorCount: 1},
			want: mappedRow{Type: "follow", Content: "关注了您!", Link: "/user/5/resource"},
		},
		{
			name: "fold",
			n:    communityclient.NotificationView{Kind: communityclient.NotificationKindFollowed, ActorID: &actor, ActorCount: 4},
			want: mappedRow{Type: "follow", Content: "等 4 人关注了您!", Link: "/user/5/resource"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := followMessage(tc.n); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestKindsMoyuNeverShowsAreNotMirrored(t *testing.T) {
	for _, kind := range []int32{
		communityclient.NotificationKindThreadCreated,
		communityclient.NotificationKindAnswerAccepted,
		communityclient.NotificationKindFeedbackStatus,
		communityclient.NotificationKindFolloweeThreadCreated,
		99,
	} {
		if mirrored(kind) {
			t.Errorf("kind %d is mirrored", kind)
		}
	}
}

func TestPartitionKeepsFollowedWithEmptyAnchor(t *testing.T) {
	actor := int64(5)
	follows, comments := partition([]communityclient.NotificationView{
		{
			ID: 11, UserID: 3, Kind: communityclient.NotificationKindFollowed,
			ActorID: &actor, ActorCount: 1, ThreadID: 0, AnchorKind: 0, AnchorID: "",
		},
		{
			ID: 12, UserID: 3, Kind: communityclient.NotificationKindFolloweeThreadCreated,
			AnchorKind: communityclient.AnchorBoard, AnchorID: "1",
		},
		{
			ID: 13, UserID: 3, Kind: communityclient.NotificationKindReplied,
			AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42",
		},
	})
	if len(follows) != 1 || follows[0].ID != 11 {
		t.Errorf("follows = %+v", follows)
	}
	if len(comments) != 1 || comments[0].ID != 13 {
		t.Errorf("comments = %+v", comments)
	}
}
