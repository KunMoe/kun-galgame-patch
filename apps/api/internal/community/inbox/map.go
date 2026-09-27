package inbox

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/pkg/communityclient"
)

const mentionExcerptRunes = 233

type mappedRow struct {
	Type    string
	Content string
	Status  int
	Link    string
}

func mirrored(kind int32) bool {
	switch kind {
	case communityclient.NotificationKindReplied, communityclient.NotificationKindMentioned,
		communityclient.NotificationKindPosted, communityclient.NotificationKindLiked:
		return true
	}
	return false
}

func partition(notes []communityclient.NotificationView) (follows, activities, comments []communityclient.NotificationView) {
	for _, n := range notes {
		switch {
		case n.Kind == communityclient.NotificationKindFollowed:
			follows = append(follows, n)
		case n.Kind == communityclient.NotificationKindFolloweeActivity:
			activities = append(activities, n)
		case mirrored(n.Kind) && anchor.IsMoyu(n.AnchorKind, n.AnchorID):
			comments = append(comments, n)
		}
	}
	return
}

func followMessage(n communityclient.NotificationView) mappedRow {
	row := mappedRow{Type: "follow", Content: "关注了您!"}
	if n.ActorCount > 1 {
		row.Content = fmt.Sprintf("等 %d 人关注了您!", n.ActorCount)
	}
	if n.ReadAt != "" {
		row.Status = 1
	}
	if n.ActorID != nil && *n.ActorID > 0 {
		row.Link = "/user/" + strconv.FormatInt(*n.ActorID, 10) + "/resource"
	}
	return row
}

// activityMessage renders a kind-10 fold from its label, title and count
// alone: kind 10 will carry object kinds this site has never heard of (the
// ones community writes for its own posts), so nothing branches on the kind.
func activityMessage(n communityclient.NotificationView) mappedRow {
	row := mappedRow{Type: "followActivity", Content: "发布了新内容"}
	if n.ReadAt != "" {
		row.Status = 1
	}
	if n.ActorID != nil && *n.ActorID > 0 {
		row.Link = "/user/" + strconv.FormatInt(*n.ActorID, 10) + "/resource"
	}
	a := n.Activity
	if a == nil {
		return row
	}
	if link := sitePath(a.URL); link != "" {
		row.Link = link
	}

	label := a.ObjectLabel
	if label == "" {
		label = "内容"
	}
	if startsLatin(label) {
		label = " " + label
	}
	switch {
	case n.ItemCount >= 100:
		row.Content = "发布了 100+ 个" + label
	case n.ItemCount > 1:
		row.Content = fmt.Sprintf("发布了 %d 个%s", n.ItemCount, label)
	default:
		row.Content = "发布了" + label
	}
	if a.Title != "" {
		sep := "："
		if n.ItemCount > 1 {
			sep = "，最新："
		}
		row.Content += sep + "「" + truncateRunes(a.Title, mentionExcerptRunes) + "」"
	}
	return row
}

// sitePath keeps an activity link inside the site. Kind 10 is delivered to the
// site the activity lives on, so its URL is always one of this site's.
func sitePath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(u.RequestURI(), "/") {
		return ""
	}
	return u.RequestURI()
}

func startsLatin(s string) bool {
	for _, r := range s {
		return r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r))
	}
	return false
}

// messageFor renders one notification as the inbox row this site shows. An
// empty excerpt means the mentioning post could not be read.
func messageFor(n communityclient.NotificationView, wall anchor.Target, excerpt string) mappedRow {
	row := mappedRow{Link: wall.Link}
	if n.PostID != nil && *n.PostID > 0 {
		row.Link += "#post-" + strconv.FormatInt(*n.PostID, 10)
	}
	if n.ReadAt != "" {
		row.Status = 1
	}

	switch n.Kind {
	case communityclient.NotificationKindReplied:
		row.Type, row.Content = "comment", "回复了您的评论"
	case communityclient.NotificationKindMentioned:
		row.Type, row.Content = "mention", "在评论中提到了您"
		if excerpt != "" {
			row.Content = truncateRunes(excerpt, mentionExcerptRunes)
		}
	case communityclient.NotificationKindPosted:
		row.Type, row.Content = "commentWatch", postedContent(n, wall)
	case communityclient.NotificationKindLiked:
		row.Type, row.Content = "likeComment", "赞了您的评论"
		if n.ActorCount > 1 {
			row.Content = fmt.Sprintf("等 %d 人赞了您的评论", n.ActorCount)
		}
	}
	return row
}

func postedContent(n communityclient.NotificationView, wall anchor.Target) string {
	var s string
	switch {
	case wall.Title == "":
		s = fmt.Sprintf("您关注的评论区有 %d 条新评论", n.ItemCount)
	case wall.ResourceID != 0:
		s = fmt.Sprintf("「%s」的补丁资源评论区有 %d 条新评论", wall.Title, n.ItemCount)
	default:
		s = fmt.Sprintf("「%s」的评论区有 %d 条新评论", wall.Title, n.ItemCount)
	}
	if n.ActorCount > 1 {
		s += fmt.Sprintf("（%d 人）", n.ActorCount)
	}
	return s
}

func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}
