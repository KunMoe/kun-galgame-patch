package service

import (
	"context"
	"fmt"
	"regexp"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/pkg/communityclient"
)

// contentImageToken is the shape a stored content image has in a comment body.
// Mirrors the regexp the reference-ping cron runs over moyu's own text columns.
var contentImageToken = regexp.MustCompile(`/image/([0-9a-f]{64})`)

const (
	sweepPageSize = 100
	// A guard, not a budget: the whole corpus is ~11k posts at 100 a page, so a
	// sweep that is still running after this many pages is a cursor that stopped
	// advancing, not a big site.
	sweepMaxPages = 500
)

// CollectImageHashes answers every content-image hash moyu's comment walls
// reference.
//
// The reference ping keeps an image from being garbage-collected, and it used
// to read patch_comment.content in SQL. Comment bodies live upstream now, so
// nothing local can see them: without this sweep every image ever posted in a
// comment would stop being referenced and eventually be collected.
//
// Posts on anchors this site does not own are skipped: the feed also answers
// catalog-anchored threads, which belong to whichever site opened them.
func (s *Service) CollectImageHashes(ctx context.Context) ([]string, error) {
	if !s.community.Configured() {
		return nil, nil
	}
	seen := make(map[string]struct{})
	cursor := ""
	for page := 0; page < sweepMaxPages; page++ {
		res, err := s.community.ListSitePosts(ctx, communityclient.SitePostsQuery{
			Kind:       communityclient.KindComments,
			AnchorKind: communityclient.AnyKind,
			Cursor:     cursor,
			Limit:      sweepPageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("community post sweep: %w", err)
		}
		for _, row := range res.Posts {
			if !anchor.IsMoyu(row.Thread.AnchorKind, row.Thread.AnchorID) {
				continue
			}
			for _, m := range contentImageToken.FindAllStringSubmatch(row.Post.ContentRaw, -1) {
				seen[m[1]] = struct{}{}
			}
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}

	out := make([]string, 0, len(seen))
	for h := range seen {
		out = append(out, h)
	}
	return out, nil
}

// PurgeAuthor is the compliance purge for one user's comments: community
// tombstones every post they wrote and deletes their reactions and read state.
//
// The per-wall counters this site caches are settled here, because the purge
// happens upstream and no SQL can recompute comment_count any more. The anchors
// are collected BEFORE the purge — afterwards the posts are tombstones and the
// author face no longer answers for them.
func (s *Service) PurgeAuthor(ctx context.Context, userID int) (int64, error) {
	if !s.community.Configured() {
		return 0, nil
	}
	perPatch := map[int]int{}
	after := ""
	for page := 0; page < sweepMaxPages; page++ {
		res, err := s.community.AuthorPosts(ctx, int64(userID), after, sweepPageSize, int(communityclient.AnyKind))
		if err != nil {
			return 0, fmt.Errorf("community author posts: %w", err)
		}
		// Resolved a page at a time: the resolver batches its resource lookup, and
		// asking per post would be a query each over everything the user wrote.
		refs := make([]anchor.Ref, 0, len(res.Posts))
		for _, row := range res.Posts {
			refs = append(refs, anchor.Ref{Kind: row.Thread.AnchorKind, ID: row.Thread.AnchorID})
		}
		targets := s.anchors.Resolve(refs)
		for _, row := range res.Posts {
			if row.Post.Status != communityclient.PostVisible {
				continue
			}
			if target, ok := targets[anchor.Ref{Kind: row.Thread.AnchorKind, ID: row.Thread.AnchorID}]; ok {
				perPatch[target.PatchID]++
			}
		}
		if res.NextCursor == "" {
			break
		}
		after = res.NextCursor
	}

	result, err := s.community.AuthorPurge(ctx, int64(userID))
	if err != nil {
		return 0, fmt.Errorf("community author purge: %w", err)
	}
	for patchID, n := range perPatch {
		s.repo.BumpCommentCount(patchID, -n)
	}
	return result.PostsPurged, nil
}

// CountAuthorComments is the purge preview's number, answered by the same face
// the profile uses.
func (s *Service) CountAuthorComments(ctx context.Context, userID int) int64 {
	return s.AuthorCounts(ctx, []int{userID})[userID]
}
