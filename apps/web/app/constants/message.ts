export const MESSAGE_TYPE = [
  'apply',
  'pm',
  'likeResource',
  'likeComment',
  'favoriteResource',
  'favorite',
  'comment',
  'commentWatch',
  'follow',
  'pr',
  'mention',
  'patchResourceCreate',
  'patchResourceUpdate',
  'system',
  ''
] as const

export const MESSAGE_TYPE_MAP: Record<string, string> = {
  apply: '申请',
  pm: '私聊',
  // Each like/favorite interaction is its own type so the message center can tell
  // them apart (resource like vs comment like vs resource favorite vs patch
  // favorite). `like` is a legacy mixed type (resource + comment likes) kept only
  // so old rows still render a label.
  likeResource: '点赞资源',
  likeComment: '点赞评论',
  like: '点赞',
  favoriteResource: '收藏资源',
  favorite: '收藏补丁',
  comment: '评论',
  commentWatch: '关注的评论区',
  follow: '关注',
  pr: '更新请求',
  mention: '提到了您',
  patchResourceCreate: '创建新补丁',
  patchResourceUpdate: '更新补丁',
  system: '系统'
}

export const MESSAGE_TYPE_ICON: Record<string, string> = {
  system: 'lucide:monitor-cog',
  pm: 'lucide:mail',
  likeComment: 'lucide:thumbs-up',
  likeResource: 'lucide:thumbs-up',
  like: 'lucide:thumbs-up',
  favoriteResource: 'lucide:heart',
  favorite: 'lucide:heart',
  comment: 'lucide:message-circle',
  commentWatch: 'lucide:message-square',
  pr: 'lucide:git-pull-request-arrow',
  follow: 'lucide:users',
  mention: 'lucide:at-sign',
  patchResourceCreate: 'lucide:plus-circle',
  patchResourceUpdate: 'lucide:refresh-cw',
  apply: 'lucide:send'
}
