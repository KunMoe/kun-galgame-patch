package communityclient

import (
	"context"
	"net/http"
)

// GetComments is a pure read: an anchor nobody has commented on yet has no
// thread, so Thread comes back nil and Posts empty. Its predecessor
// POST /comments/resolve was get-or-create, and because every site called it to
// RENDER a page it had minted 110,918 empty threads by 2026-09-15 — 97.2% of
// every thread in the community database. Never call resolve from a read path.
//
// `after` is a post_number, not an opaque cursor: this face keysets on the
// thread's own numbering.
func (c *Client) GetComments(ctx context.Context, anchorKind int32, anchorID, after, limit string, viewerID int64) (*CommentsPage, error) {
	var out CommentsPage
	q := query(map[string]string{
		"anchor_kind": itoa(int64(anchorKind)), "anchor_id": anchorID, "after": after, "limit": limit,
		"viewer_id": viewer(viewerID),
	})
	err := c.do(ctx, "getComments", http.MethodGet, "/comments"+q, nil, &out)
	return &out, err
}

// CommentOnAnchor posts to an anchor's comment wall; the thread is created in
// the same transaction when this is its first comment.
//
// A repeated idempotencyKey answers with the post the first call wrote, and
// writes nothing (infra #302). Community scopes a key to the site, not the
// user, so the key must already be the author's own; an empty one always
// writes.
func (c *Client) CommentOnAnchor(ctx context.Context, req CommentRequest, idempotencyKey string) (*ThreadWithPost, error) {
	var out ThreadWithPost
	replayed, err := c.send(ctx, "comment", http.MethodPost, "/comments", idempotencyKey, req, &out)
	out.Replayed = replayed
	return &out, err
}

func (c *Client) EditPost(ctx context.Context, postID int64, req EditPostRequest) (*PostView, error) {
	var out struct {
		Post PostView `json:"post"`
	}
	err := c.do(ctx, "editPost", http.MethodPatch, "/posts/"+itoa(postID), req, &out)
	return &out.Post, err
}

func (c *Client) DeletePost(ctx context.Context, postID, authorID int64, asModerator bool) error {
	q := map[string]string{"author_id": itoa(authorID)}
	if asModerator {
		q["as_moderator"] = "true"
	}
	return c.do(ctx, "deletePost", http.MethodDelete, "/posts/"+itoa(postID)+query(q), nil, nil)
}

// SetReaction and UnsetReaction replace the toggle, which undid itself when a
// timed-out click was retried. Repeating either changes nothing and answers
// Changed false.
func (c *Client) SetReaction(ctx context.Context, postID, userID int64, kind int32) (*ReactionResult, error) {
	var out ReactionResult
	req := ReactionRequest{UserID: userID, Kind: kind}
	err := c.do(ctx, "setReaction", http.MethodPut, "/posts/"+itoa(postID)+"/reaction", req, &out)
	return &out, err
}

func (c *Client) UnsetReaction(ctx context.Context, postID, userID int64, kind int32) (*ReactionResult, error) {
	var out ReactionResult
	q := query(map[string]string{"user_id": itoa(userID), "kind": itoa(int64(kind))})
	err := c.do(ctx, "unsetReaction", http.MethodDelete, "/posts/"+itoa(postID)+"/reaction"+q, nil, &out)
	return &out, err
}

func (c *Client) SubmitFlag(ctx context.Context, postID int64, req FlagRequest) error {
	return c.do(ctx, "submitFlag", http.MethodPost, "/posts/"+itoa(postID)+"/flag", req, nil)
}

// ResolvePosts is the id-addressed read: it answers each post with the thread
// context its anchor needs, which is how a post id alone resolves to a page.
func (c *Client) ResolvePosts(ctx context.Context, ids []int64, viewerID int64) (*PostsResolveResponse, error) {
	if len(ids) == 0 {
		return &PostsResolveResponse{Posts: []AuthorPostView{}}, nil
	}
	var out PostsResolveResponse
	err := c.do(ctx, "resolvePosts", http.MethodPost, "/posts/resolve", PostsResolveRequest{IDs: ids, ViewerID: viewerID}, &out)
	return &out, err
}

// AuthorPosts keysets by id, which is the existing contract for this face.
// The site feed and search below keyset by creation time instead — the moyu
// import gives historical comments fresh ids, so id order is import order.
func (c *Client) AuthorPosts(ctx context.Context, authorID int64, after string, limit, anchorKind int) (*AuthorPostsResponse, error) {
	var out AuthorPostsResponse
	q := map[string]string{"after": after}
	if limit > 0 {
		q["limit"] = itoa(int64(limit))
	}
	if anchorKind >= 0 {
		q["anchor_kind"] = itoa(int64(anchorKind))
	}
	err := c.do(ctx, "listAuthorPosts", http.MethodGet, "/authors/"+itoa(authorID)+"/posts"+query(q), nil, &out)
	return &out, err
}

func (c *Client) AuthorStats(ctx context.Context, ids []int64, kind, anchorKind int32) (*AuthorStatsResponse, error) {
	if len(ids) == 0 {
		return &AuthorStatsResponse{Stats: []AuthorStat{}}, nil
	}
	var out AuthorStatsResponse
	q := query(map[string]string{
		"ids": joinInt64(ids), "kind": itoa(int64(kind)), "anchor_kind": itoa(int64(anchorKind)),
	})
	err := c.do(ctx, "authorStats", http.MethodGet, "/authors/stats"+q, nil, &out)
	return &out, err
}

// TopAuthors ranks the site's authors by visible posts, most first. AuthorStats
// answers a caller that already knows whose numbers it wants; this is the
// question a leaderboard asks, which no batch of named ids can.
func (c *Client) TopAuthors(ctx context.Context, kind, anchorKind int32, limit int) (*AuthorStatsResponse, error) {
	var out AuthorStatsResponse
	q := map[string]string{"kind": itoa(int64(kind)), "anchor_kind": itoa(int64(anchorKind))}
	if limit > 0 {
		q["limit"] = itoa(int64(limit))
	}
	err := c.do(ctx, "topAuthors", http.MethodGet, "/authors/top"+query(q), nil, &out)
	return &out, err
}

func (c *Client) AuthorPurge(ctx context.Context, authorID int64) (*PurgeResult, error) {
	var out PurgeResult
	err := c.do(ctx, "purgeAuthor", http.MethodPost, "/authors/"+itoa(authorID)+"/purge", nil, &out)
	return &out, err
}

// SitePostsQuery narrows the site-wide feed. A zero Kind/AnchorKind is "every
// kind" upstream only when sent as -1, so both carry an explicit "any" here.
type SitePostsQuery struct {
	Kind        int32
	AnchorKind  int32
	AnchorID    string
	RepliesOnly bool
	Cursor      string
	Limit       int
}

const AnyKind int32 = -1

// ListSitePosts is the newest-posts-across-every-thread face. It keysets by
// creation time, not id.
func (c *Client) ListSitePosts(ctx context.Context, q SitePostsQuery) (*PostFeedResponse, error) {
	var out PostFeedResponse
	params := map[string]string{
		"kind":        itoa(int64(q.Kind)),
		"anchor_kind": itoa(int64(q.AnchorKind)),
		"anchor_id":   q.AnchorID,
		"cursor":      q.Cursor,
	}
	if q.RepliesOnly {
		params["replies_only"] = "true"
	}
	if q.Limit > 0 {
		params["limit"] = itoa(int64(q.Limit))
	}
	err := c.do(ctx, "listSitePosts", http.MethodGet, "/posts"+query(params), nil, &out)
	return &out, err
}

// SearchPosts matches the markdown source, not the cooked HTML, and answers a
// keyset page: there is no total to count and no relevance to rank by.
func (c *Client) SearchPosts(ctx context.Context, q string, kind int32, cursor string, limit int) (*PostFeedResponse, error) {
	var out PostFeedResponse
	params := map[string]string{"q": q, "kind": itoa(int64(kind)), "cursor": cursor}
	if limit > 0 {
		params["limit"] = itoa(int64(limit))
	}
	err := c.do(ctx, "searchPosts", http.MethodGet, "/search/posts"+query(params), nil, &out)
	return &out, err
}

// MarkThreadRead advances the reader's high-water mark; it is clamped upstream
// to the thread's highest post number, so MaxInt32 means "all of it".
func (c *Client) MarkThreadRead(ctx context.Context, threadID, userID int64, lastRead int32) (*ThreadUserView, error) {
	var out ThreadUserView
	req := ThreadReadRequest{UserID: userID, LastReadPostNumber: lastRead}
	err := c.do(ctx, "markThreadRead", http.MethodPost, "/threads/"+itoa(threadID)+"/read", req, &out)
	return &out, err
}

func (c *Client) SetThreadNotification(ctx context.Context, threadID, userID int64, level int32) (*ThreadUserView, error) {
	var out ThreadUserView
	req := ThreadNotificationRequest{UserID: userID, Level: level}
	err := c.do(ctx, "setThreadNotification", http.MethodPost, "/threads/"+itoa(threadID)+"/notification", req, &out)
	return &out, err
}

// ThreadStates reports only the threads the user has actually interacted with:
// a thread they never opened carries no row and is simply absent.
func (c *Client) ThreadStates(ctx context.Context, userID int64, threadIDs []int64) (*ThreadStatesResponse, error) {
	if len(threadIDs) == 0 {
		return &ThreadStatesResponse{States: []ThreadUserView{}}, nil
	}
	var out ThreadStatesResponse
	err := c.do(ctx, "threadStates", http.MethodPost, "/threads/states", ThreadStatesRequest{UserID: userID, ThreadIDs: threadIDs}, &out)
	return &out, err
}

func (c *Client) ListUnread(ctx context.Context, userID int64, cursor string, limit int) (*UnreadListResponse, error) {
	var out UnreadListResponse
	q := map[string]string{"cursor": cursor}
	if limit > 0 {
		q["limit"] = itoa(int64(limit))
	}
	err := c.do(ctx, "listUnread", http.MethodGet, "/users/"+itoa(userID)+"/unread"+query(q), nil, &out)
	return &out, err
}

func (c *Client) NotificationFeed(ctx context.Context, after int64, limit int) (*NotificationFeedResponse, error) {
	var out NotificationFeedResponse
	q := map[string]string{"after": itoa(after)}
	if limit > 0 {
		q["limit"] = itoa(int64(limit))
	}
	err := c.do(ctx, "notificationFeed", http.MethodGet, "/notifications/feed"+query(q), nil, &out)
	return &out, err
}

func (c *Client) SetAnchorNotification(ctx context.Context, userID int64, anchorKind int32, anchorID string, level int32) (*AnchorStateView, error) {
	var out AnchorStateView
	req := AnchorNotificationRequest{UserID: userID, AnchorKind: anchorKind, AnchorID: anchorID, Level: level}
	err := c.do(ctx, "setAnchorNotification", http.MethodPost, "/anchors/notification", req, &out)
	return &out, err
}

func (c *Client) AnchorStates(ctx context.Context, userID int64, anchors []AnchorRef) (*AnchorStatesResponse, error) {
	if len(anchors) == 0 {
		return &AnchorStatesResponse{States: []AnchorStateView{}}, nil
	}
	var out AnchorStatesResponse
	err := c.do(ctx, "anchorStates", http.MethodPost, "/anchors/states", AnchorStatesRequest{UserID: userID, Anchors: anchors}, &out)
	return &out, err
}

func (c *Client) MarkNotificationsRead(ctx context.Context, userID int64, ids []int64) (*MarkNotificationsReadResult, error) {
	var out MarkNotificationsReadResult
	err := c.do(ctx, "markNotificationsRead", http.MethodPost, "/users/"+itoa(userID)+"/notifications/read", MarkNotificationsReadRequest{IDs: ids}, &out)
	return &out, err
}

func (c *Client) FollowUser(ctx context.Context, userID, targetID int64) (*FollowResult, error) {
	var out FollowResult
	err := c.do(ctx, "followUser", http.MethodPut, "/users/"+itoa(userID)+"/following/"+itoa(targetID), nil, &out)
	return &out, err
}

func (c *Client) UnfollowUser(ctx context.Context, userID, targetID int64) (*FollowResult, error) {
	var out FollowResult
	err := c.do(ctx, "unfollowUser", http.MethodDelete, "/users/"+itoa(userID)+"/following/"+itoa(targetID), nil, &out)
	return &out, err
}

func (c *Client) ListFollowers(ctx context.Context, userID int64, cursor string, limit int) (*FollowListResponse, error) {
	return c.listFollows(ctx, "listFollowers", "/users/"+itoa(userID)+"/followers", cursor, limit)
}

func (c *Client) ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (*FollowListResponse, error) {
	return c.listFollows(ctx, "listFollowing", "/users/"+itoa(userID)+"/following", cursor, limit)
}

func (c *Client) listFollows(ctx context.Context, op, path, cursor string, limit int) (*FollowListResponse, error) {
	var out FollowListResponse
	q := map[string]string{"cursor": cursor}
	if limit > 0 {
		q["limit"] = itoa(int64(limit))
	}
	err := c.do(ctx, op, http.MethodGet, path+query(q), nil, &out)
	return &out, err
}

const followStatesBatch = 100

func (c *Client) FollowStates(ctx context.Context, viewerID int64, userIDs []int64) (*FollowStatesResponse, error) {
	if len(userIDs) == 0 {
		return &FollowStatesResponse{States: []FollowStateView{}}, nil
	}
	out := FollowStatesResponse{States: make([]FollowStateView, 0, len(userIDs))}
	for start := 0; start < len(userIDs); start += followStatesBatch {
		end := min(start+followStatesBatch, len(userIDs))
		req := FollowStatesRequest{UserIDs: userIDs[start:end]}
		if viewerID > 0 {
			req.ViewerID = viewerID
		}
		var page FollowStatesResponse
		if err := c.do(ctx, "followStates", http.MethodPost, "/follows/states", req, &page); err != nil {
			return nil, err
		}
		out.States = append(out.States, page.States...)
	}
	return &out, nil
}
