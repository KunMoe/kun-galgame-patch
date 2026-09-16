import { kunMoyuMoe } from '~/config/moyu-moe'

// What a comment area is attached to, and everything that differs between the
// two areas — modelled on kungal's utils/communityComment.ts, which is why all
// six of its comment sections look identical: the LOOK lives in one component
// set, and only the addressing is per-surface.
//
// Both walls are threads in the NextMoe community primitive. moyu addresses them
// by ANCHOR, not by thread id, and the anchor is minted server-side:
//   patch    → anchor_kind=1 "<patch id>"    (/galgame/:id?tab=comment)
//   resource → anchor_kind=2 "<resource id>" (/resource/:rid)

export type CommentTarget =
  | { kind: 'patch'; galgameId: number }
  // galgameId is carried for the resource area too: a report's evidence and the
  // mention policy are patch-scoped, and the composer's 未收录 wording needs it.
  | { kind: 'resource'; resourceId: number; galgameId: number }

export interface CommentSurface {
  kind: CommentTarget['kind']
  // Keyset list read. Both areas take the same ?after=&limit=, where `after` is
  // a post_number rather than an opaque cursor.
  listUrl: string
  // POST target; the body is { content, reply_to_post_id? } for both.
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
  // The resource this area belongs to, or null for the patch area.
  resourceId: number | null
}

// The anchor id a comment node renders and a new deep-link targets. It is the
// POST id, and the `post-` prefix is what keeps it apart from the pre-cutover
// `comment-<n>` shape: an old comment id and a post id can be the same number,
// and the two resolve through completely different paths.
export const commentAnchorId = (postId: number) => `post-${postId}`

// Links minted before the cutover. They still arrive from notifications and from
// anywhere a reader saved one, and only the server's map table can resolve them.
export const legacyCommentAnchorId = (commentId: number) => `comment-${commentId}`

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

// Absolute URL of one comment — the evidence link handed to the report flow, so
// a moderator opens the comment in context on whichever surface it lives.
export const commentAbsoluteUrl = (
  surface: CommentSurface,
  postId: number
) => `${kunMoyuMoe.domain.main}${surface.pagePath}#${commentAnchorId(postId)}`

// The report reasons the community service accepts, in its own numbering.
export const COMMENT_FLAG_REASONS = [
  { value: 0, label: '垃圾广告' },
  { value: 1, label: '辱骂 / 人身攻击' },
  { value: 2, label: '与主题无关' },
  { value: 4, label: 'NSFW 标注错误' },
  { value: 3, label: '其他' }
] as const
