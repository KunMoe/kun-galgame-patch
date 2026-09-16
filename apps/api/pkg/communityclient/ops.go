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
	err := c.do(ctx, http.MethodGet, "/comments"+q, nil, &out)
	return &out, err
}

// CommentOnAnchor posts to an anchor's comment wall; the thread is created in
// the same transaction when this is its first comment.
func (c *Client) CommentOnAnchor(ctx context.Context, req CommentRequest) (*ThreadWithPost, error) {
	var out ThreadWithPost
	err := c.do(ctx, http.MethodPost, "/comments", req, &out)
	return &out, err
}

func (c *Client) EditPost(ctx context.Context, postID int64, req EditPostRequest) (*PostView, error) {
	var out struct {
		Post PostView `json:"post"`
	}
	err := c.do(ctx, http.MethodPatch, "/posts/"+itoa(postID), req, &out)
	return &out.Post, err
}

func (c *Client) DeletePost(ctx context.Context, postID, authorID int64, asModerator bool) error {
	q := map[string]string{"author_id": itoa(authorID)}
	if asModerator {
		q["as_moderator"] = "true"
	}
	return c.do(ctx, http.MethodDelete, "/posts/"+itoa(postID)+query(q), nil, nil)
}

func (c *Client) ToggleReaction(ctx context.Context, postID int64, req ReactionToggleRequest) (*ReactionToggleResult, error) {
	var out ReactionToggleResult
	err := c.do(ctx, http.MethodPost, "/posts/"+itoa(postID)+"/reaction", req, &out)
	return &out, err
}

func (c *Client) SubmitFlag(ctx context.Context, postID int64, req FlagRequest) error {
	return c.do(ctx, http.MethodPost, "/posts/"+itoa(postID)+"/flag", req, nil)
}

// ResolvePosts is the id-addressed read: it answers each post with the thread
// context its anchor needs, which is how a post id alone resolves to a page.
func (c *Client) ResolvePosts(ctx context.Context, ids []int64, viewerID int64) (*PostsResolveResponse, error) {
	if len(ids) == 0 {
		return &PostsResolveResponse{Posts: []AuthorPostView{}}, nil
	}
	var out PostsResolveResponse
	err := c.do(ctx, http.MethodPost, "/posts/resolve", PostsResolveRequest{IDs: ids, ViewerID: viewerID}, &out)
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
	err := c.do(ctx, http.MethodGet, "/authors/"+itoa(authorID)+"/posts"+query(q), nil, &out)
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
	err := c.do(ctx, http.MethodGet, "/authors/stats"+q, nil, &out)
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
	err := c.do(ctx, http.MethodGet, "/authors/top"+query(q), nil, &out)
	return &out, err
}

func (c *Client) AuthorPurge(ctx context.Context, authorID int64) (*PurgeResult, error) {
	var out PurgeResult
	err := c.do(ctx, http.MethodPost, "/authors/"+itoa(authorID)+"/purge", nil, &out)
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
	err := c.do(ctx, http.MethodGet, "/posts"+query(params), nil, &out)
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
	err := c.do(ctx, http.MethodGet, "/search/posts"+query(params), nil, &out)
	return &out, err
}

// MarkThreadRead advances the reader's high-water mark; it is clamped upstream
// to the thread's highest post number, so MaxInt32 means "all of it".
func (c *Client) MarkThreadRead(ctx context.Context, threadID, userID int64, lastRead int32) (*ThreadUserView, error) {
	var out ThreadUserView
	req := ThreadReadRequest{UserID: userID, LastReadPostNumber: lastRead}
	err := c.do(ctx, http.MethodPost, "/threads/"+itoa(threadID)+"/read", req, &out)
	return &out, err
}

func (c *Client) SetThreadNotification(ctx context.Context, threadID, userID int64, level int32) (*ThreadUserView, error) {
	var out ThreadUserView
	req := ThreadNotificationRequest{UserID: userID, Level: level}
	err := c.do(ctx, http.MethodPost, "/threads/"+itoa(threadID)+"/notification", req, &out)
	return &out, err
}

// ThreadStates reports only the threads the user has actually interacted with:
// a thread they never opened carries no row and is simply absent.
func (c *Client) ThreadStates(ctx context.Context, userID int64, threadIDs []int64) (*ThreadStatesResponse, error) {
	if len(threadIDs) == 0 {
		return &ThreadStatesResponse{States: []ThreadUserView{}}, nil
	}
	var out ThreadStatesResponse
	err := c.do(ctx, http.MethodPost, "/threads/states", ThreadStatesRequest{UserID: userID, ThreadIDs: threadIDs}, &out)
	return &out, err
}

func (c *Client) ListUnread(ctx context.Context, userID int64, cursor string, limit int) (*UnreadListResponse, error) {
	var out UnreadListResponse
	q := map[string]string{"cursor": cursor}
	if limit > 0 {
		q["limit"] = itoa(int64(limit))
	}
	err := c.do(ctx, http.MethodGet, "/users/"+itoa(userID)+"/unread"+query(q), nil, &out)
	return &out, err
}
