// Label maps for the Trust & Safety moderation inbox (integer codes → zh-cn).
type ChipColor = 'warning' | 'primary' | 'danger' | 'default'

export const TRUST_REVIEW_STATUS: Record<
  number,
  { label: string; color: ChipColor }
> = {
  0: { label: '待处理', color: 'warning' },
  1: { label: '处理中', color: 'primary' },
  2: { label: '已处置', color: 'danger' },
  3: { label: '已驳回', color: 'default' }
}

export const TRUST_REVIEW_SOURCE: Record<number, string> = {
  0: '用户举报',
  1: 'AI 文本',
  2: 'AI 图片',
  3: '社区转入',
  4: '分级纠错',
  5: '人工新建'
}

// Disposition actions offered when a moderator decides `actioned`.
export const TRUST_ACTIONS = [
  { value: 1, label: '隐藏内容' },
  { value: 2, label: '删除内容' },
  { value: 3, label: '警告作者' },
  { value: 4, label: '限制用户' },
  { value: 5, label: '升级至账号中心' },
  { value: 0, label: '不处置（仅记录）' }
] as const

// moyu's reportable subject kinds (mirror the backend enforce registry).
export const TRUST_SUBJECT_KIND: Record<string, string> = {
  patch_comment: '补丁评论',
  patch_resource: '补丁资源',
  user: '用户'
}

// A page URL for a subject when its id maps to a standalone page. patch_comment
// has no page of its own (it's a child of a patch), so undefined → the console
// falls back to the reporter-carried subject_url.
export const trustSubjectHref = (
  kind: string,
  id: string
): string | undefined => {
  switch (kind) {
    case 'patch_resource':
      return `/resource/${id}`
    case 'user':
      return `/user/${id}`
    default:
      return undefined
  }
}
