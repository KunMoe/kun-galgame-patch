import { kunMoyuMoe } from '~/config/moyu-moe'

// What a comment area is attached to, and everything that differs between the
// two areas — modelled on kungal's utils/communityComment.ts, which is why all
// six of its comment sections look identical: the LOOK lives in one component
// set, and only the addressing is per-surface.
//
// moyu has two areas:
//   patch    → /patch/:gid/comment   (patch_comment.resource_id IS NULL)
//   resource → /resource/:rid        (patch_comment.resource_id = :rid)
//
// Both are the same table and the same wire shape, so edit / delete / like /
// locate are comment-addressed and shared; only list + create + the deep-link
// page differ.

export type CommentTarget =
  | { kind: 'patch'; galgameId: number }
  // galgameId is carried for the resource area too: a report's evidence and the
  // mention policy are patch-scoped, and the composer's 未收录 wording needs it.
  | { kind: 'resource'; resourceId: number; galgameId: number }

export interface CommentSurface {
  kind: CommentTarget['kind']
  // Paginated list read. Both areas take the same ?page=&limit=.
  listUrl: string
  // POST target; the body is { content, parent_id? } for both.
  createUrl: string
  // Absolute page a comment lives on, WITHOUT the anchor — the base for the
  // report evidence URL and for the "jump here" deep-link.
  pagePath: string
  emptyDescription: string
  composerPlaceholder: string
  // Standing notice above the composer. The resource area needs one that the
  // placeholder can't provide: its composer is pre-seeded with the publisher's
  // @mention, so it is never empty and the placeholder never renders.
  notice: { title: string; body: string } | null
  // The resource this area belongs to, or null for the patch area. Used to
  // reject a locate() result that belongs to the OTHER area.
  resourceId: number | null
}

// The anchor id every comment node renders and every deep-link targets. Shared
// verbatim across both areas so links minted before resource comments existed
// (#comment-<id>) keep resolving.
export const commentAnchorId = (commentId: number) => `comment-${commentId}`

export const commentSurface = (target: CommentTarget): CommentSurface => {
  if (target.kind === 'resource') {
    return {
      kind: 'resource',
      listUrl: `/patch/resource/${target.resourceId}/comment`,
      createUrl: `/patch/resource/${target.resourceId}/comment`,
      pagePath: `/resource/${target.resourceId}`,
      emptyDescription: '还没有人评论这个资源, 用得怎么样, 说两句吧~',
      composerPlaceholder:
        '这个资源用得怎么样？安装体验、链接是否有效、解压密码是否正确都可以在这里反馈，发布者会收到通知～',
      notice: {
        title: '资源有问题？就在这里反馈',
        body: '下载链接失效、提取码或解压密码错误、文件损坏、解压后无法运行、补丁与游戏版本不匹配……都请直接在下面留言。评论会 @ 到资源发布者，他会收到通知并来处理，这里是最快能解决问题的地方。反馈时请带上你的系统、游戏版本和具体报错，方便定位。'
      },
      resourceId: target.resourceId
    }
  }
  return {
    kind: 'patch',
    listUrl: `/patch/${target.galgameId}/comment`,
    createUrl: `/patch/${target.galgameId}/comment`,
    pagePath: `/galgame/${target.galgameId}?tab=comment`,
    emptyDescription: '暂无评论, 快来抢沙发吧~',
    // Routes resource complaints to the right place. Now that every resource has
    // its own comment area, a "链接失效了" posted here reaches nobody who can act
    // on it — the resource's publisher is only notified on THEIR area.
    composerPlaceholder:
      '如果资源有问题（下载链接失效、解压密码错误、文件损坏等），请前往那个资源的详情页面，在它的评论区反馈，发布者会收到通知。本评论区仅用于对游戏本身的评价与讨论，不处理资源问题～',
    // The patch area's guidance fits in the placeholder (its composer starts
    // empty), so no standing banner — one more box above every game's comments
    // would be noise on the surface that isn't about resources.
    notice: null,
    resourceId: null
  }
}

// Absolute URL of one comment — the evidence link handed to the report modal, so
// a moderator opens the comment in context on whichever surface it lives.
export const commentAbsoluteUrl = (
  surface: CommentSurface,
  commentId: number
) => `${kunMoyuMoe.domain.main}${surface.pagePath}#${commentAnchorId(commentId)}`

// Site-relative permalink for a comment ROW coming out of any of the mixed feeds
// (home / the global feed / a user's profile / the admin queue). Those lists
// contain both kinds, so the surface has to be decided per row: a resource
// comment is NOT reachable at /patch/:gid/comment, whose list filters
// resource_id IS NULL, so the patch shape would land the reader on a page the
// comment isn't on. Mirrors the server's commentAnchorLink.
export const commentPermalink = (comment: {
  id: number
  galgame_id: number
  resource_id?: number | null
}) =>
  comment.resource_id
    ? `/resource/${comment.resource_id}#${commentAnchorId(comment.id)}`
    : `/galgame/${comment.galgame_id}?tab=comment#${commentAnchorId(comment.id)}`
