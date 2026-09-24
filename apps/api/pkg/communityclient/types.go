package communityclient

const (
	AnchorBoard         = 0
	AnchorSiteGame      = 1
	AnchorSiteResource  = 2
	AnchorCatalogWork   = 3
	AnchorCatalogPerson = 4
)

const (
	KindTopic    = 0
	KindComments = 1
	KindFeedback = 2
)

const (
	RatingAll = 0
	RatingR15 = 1
	RatingR18 = 2
)

const ReactionLike = 0

const (
	PostVisible = 0
	PostHeld    = 1
	PostDeleted = 2
)

const (
	NotificationMuted             = 0
	NotificationNormal            = 1
	NotificationTracking          = 2
	NotificationWatching          = 3
	NotificationWatchingFirstPost = 4
)

const (
	NotificationKindReplied        = 1
	NotificationKindMentioned      = 2
	NotificationKindPosted         = 3
	NotificationKindThreadCreated  = 4
	NotificationKindLiked          = 5
	NotificationKindAnswerAccepted = 6
	NotificationKindFeedbackStatus = 7
)

const (
	FlagReasonSpam         = 0
	FlagReasonAbuse        = 1
	FlagReasonOffTopic     = 2
	FlagReasonOther        = 3
	FlagReasonNsfwMislabel = 4
)

type PostView struct {
	ID                int64  `json:"id"`
	ThreadID          int64  `json:"thread_id"`
	PostNumber        int32  `json:"post_number"`
	AuthorID          int64  `json:"author_id"`
	ContentRaw        string `json:"content_raw"`
	ContentHTML       string `json:"content_html"`
	ContentRating     int32  `json:"content_rating"`
	Status            int32  `json:"status"`
	CreatedAt         string `json:"created_at"`
	EditedAt          string `json:"edited_at"`
	EditedByModerator bool   `json:"edited_by_moderator"`
	ReplyToPostID     int64  `json:"reply_to_post_id"`
	RootPostID        int64  `json:"root_post_id"`
	TargetUserID      int64  `json:"target_user_id"`

	// ReactionCount is filled by every face that returns a post it did not just
	// create. PATCH /posts/{id} answered 0 until infra cce5b4a8 (2026-09-16).
	ReactionCount int32 `json:"reaction_count"`
	// ViewerReacted needs a viewer: viewer_id on a read, the acting author_id on
	// an edit.
	ViewerReacted bool `json:"viewer_reacted"`
}

type ThreadView struct {
	ID                int64  `json:"id"`
	Site              string `json:"site"`
	Kind              int32  `json:"kind"`
	AnchorKind        int32  `json:"anchor_kind"`
	AnchorID          string `json:"anchor_id"`
	ContentRating     int32  `json:"content_rating"`
	Status            int32  `json:"status"`
	PostsCount        int32  `json:"posts_count"`
	ParticipantsCount int32  `json:"participants_count"`
	HighestPostNumber int32  `json:"highest_post_number"`
	CreatedBy         int64  `json:"created_by"`
	CreatedAt         string `json:"created_at"`
	LastPostedAt      string `json:"last_posted_at"`
}

// CommentsPage is the anchor read face. Thread is nil until somebody comments:
// the wall is not created by looking at it.
type CommentsPage struct {
	Thread     *ThreadView `json:"thread"`
	Posts      []PostView  `json:"posts"`
	NextCursor string      `json:"next_cursor"`
}

type CommentRequest struct {
	AnchorKind     int32   `json:"anchor_kind"`
	AnchorID       string  `json:"anchor_id"`
	ContentRating  int32   `json:"content_rating"`
	AuthorID       int64   `json:"author_id"`
	Body           string  `json:"body"`
	RootPostID     int64   `json:"root_post_id,omitempty"`
	ReplyToPostID  int64   `json:"reply_to_post_id,omitempty"`
	TargetUserID   int64   `json:"target_user_id,omitempty"`
	MentionUserIDs []int64 `json:"mention_user_ids,omitempty"`
}

type ThreadWithPost struct {
	Thread ThreadView `json:"thread"`
	Post   PostView   `json:"post"`
	// Replayed is an earlier request's answer, repeated for the same key: the
	// post already existed and its side effects already ran.
	Replayed bool `json:"-"`
}

type PostListResponse struct {
	Posts      []PostView `json:"posts"`
	NextCursor string     `json:"next_cursor"`
}

type PostThreadContext struct {
	ThreadID   int64  `json:"thread_id"`
	Title      string `json:"title"`
	AnchorKind int32  `json:"anchor_kind"`
	AnchorID   string `json:"anchor_id"`
}

type AuthorPostView struct {
	Post   PostView          `json:"post"`
	Thread PostThreadContext `json:"thread"`
}

type AuthorPostsResponse struct {
	Posts      []AuthorPostView `json:"posts"`
	NextCursor string           `json:"next_cursor"`
}

type PostFeedResponse struct {
	Posts      []AuthorPostView `json:"posts"`
	NextCursor string           `json:"next_cursor"`
}

type PostsResolveRequest struct {
	IDs      []int64 `json:"ids"`
	ViewerID int64   `json:"viewer_id,omitempty"`
}

type PostsResolveResponse struct {
	Posts []AuthorPostView `json:"posts"`
}

type ReplyRequest struct {
	AuthorID      int64  `json:"author_id"`
	Body          string `json:"body"`
	ReplyToPostID int64  `json:"reply_to_post_id,omitempty"`
	TargetUserID  int64  `json:"target_user_id,omitempty"`
}

type EditPostRequest struct {
	AuthorID    int64  `json:"author_id"`
	Body        string `json:"body"`
	AsModerator bool   `json:"as_moderator,omitempty"`
}

type ReactionRequest struct {
	UserID int64 `json:"user_id"`
	Kind   int32 `json:"kind"`
}

type ReactionResult struct {
	Added         bool   `json:"added"`
	Changed       bool   `json:"changed"`
	AuthorID      int64  `json:"author_id"`
	ThreadID      int64  `json:"thread_id"`
	AnchorKind    int32  `json:"anchor_kind"`
	AnchorID      string `json:"anchor_id"`
	ReactionCount int32  `json:"reaction_count"`
}

type FlagRequest struct {
	FlaggerID int64  `json:"flagger_id"`
	Reason    int32  `json:"reason"`
	Note      string `json:"note,omitempty"`
}

type AuthorStat struct {
	AuthorID     int64 `json:"author_id"`
	VisiblePosts int64 `json:"visible_posts"`
}

type AuthorStatsResponse struct {
	Stats []AuthorStat `json:"stats"`
}

type PurgeResult struct {
	PostsPurged       int64 `json:"posts_purged"`
	ReactionsDeleted  int64 `json:"reactions_deleted"`
	ReadStatesDeleted int64 `json:"read_states_deleted"`
}

type ThreadUserView struct {
	ThreadID           int64 `json:"thread_id"`
	UserID             int64 `json:"user_id"`
	LastReadPostNumber int32 `json:"last_read_post_number"`
	HighestPostNumber  int32 `json:"highest_post_number"`
	UnreadCount        int32 `json:"unread_count"`
	NotificationLevel  int32 `json:"notification_level"`
}

type ThreadReadRequest struct {
	UserID             int64 `json:"user_id"`
	LastReadPostNumber int32 `json:"last_read_post_number"`
}

type ThreadNotificationRequest struct {
	UserID int64 `json:"user_id"`
	Level  int32 `json:"level"`
}

type ThreadStatesRequest struct {
	UserID    int64   `json:"user_id"`
	ThreadIDs []int64 `json:"thread_ids"`
}

type ThreadStatesResponse struct {
	States []ThreadUserView `json:"states"`
}

type UnreadThreadView struct {
	Thread ThreadView     `json:"thread"`
	State  ThreadUserView `json:"state"`
}

type UnreadListResponse struct {
	Threads    []UnreadThreadView `json:"threads"`
	NextCursor string             `json:"next_cursor"`
	Total      int64              `json:"total"`
}

type NotificationView struct {
	ID              int64  `json:"id"`
	UserID          int64  `json:"user_id"`
	Kind            int32  `json:"kind"`
	ThreadID        int64  `json:"thread_id"`
	AnchorKind      int32  `json:"anchor_kind"`
	AnchorID        string `json:"anchor_id"`
	BoardID         int64  `json:"board_id"`
	PostID          *int64 `json:"post_id"`
	PostNumber      *int32 `json:"post_number"`
	FirstPostNumber *int32 `json:"first_post_number"`
	ActorID         *int64 `json:"actor_id"`
	ActorCount      int32  `json:"actor_count"`
	ItemCount       int32  `json:"item_count"`
	ReadAt          string `json:"read_at"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Seq             int64  `json:"seq"`
}

type NotificationFeedResponse struct {
	Notifications []NotificationView `json:"notifications"`
	NextAfter     int64              `json:"next_after"`
}

type AnchorRef struct {
	AnchorKind int32  `json:"anchor_kind"`
	AnchorID   string `json:"anchor_id"`
}

type AnchorNotificationRequest struct {
	UserID     int64  `json:"user_id"`
	AnchorKind int32  `json:"anchor_kind"`
	AnchorID   string `json:"anchor_id"`
	Level      int32  `json:"level"`
}

type AnchorStateView struct {
	UserID            int64  `json:"user_id"`
	AnchorKind        int32  `json:"anchor_kind"`
	AnchorID          string `json:"anchor_id"`
	NotificationLevel int32  `json:"notification_level"`
}

type AnchorStatesRequest struct {
	UserID  int64       `json:"user_id"`
	Anchors []AnchorRef `json:"anchors"`
}

type AnchorStatesResponse struct {
	States []AnchorStateView `json:"states"`
}

type MarkNotificationsReadRequest struct {
	IDs []int64 `json:"ids"`
}

type MarkNotificationsReadResult struct {
	Marked      int64 `json:"marked"`
	UnreadCount int64 `json:"unread_count"`
}
