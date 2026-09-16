package inbox

import (
	"fmt"
	"strconv"

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
