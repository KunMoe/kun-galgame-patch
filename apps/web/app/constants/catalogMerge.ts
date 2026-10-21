export const MERGE_ACTION_LABEL: Record<string, string> = {
  renumber: '页面改号',
  fold: '页面折叠',
  ledger: '仅补跳转'
}

export const MERGE_TABLE_LABEL: Record<string, string> = {
  patch_resource: '补丁资源',
  patch_comment: '旧评论',
  patch_link: '外链',
  user_patch_contribute_relation: '贡献者',
  user_patch_favorite_relation: '旧收藏',
  patch_redirect: '跳转'
}

export const mergeTableCounts = (counts?: Record<string, number | number[]>) =>
  Object.entries(counts ?? {})
    .map(
      ([table, v]) =>
        `${MERGE_TABLE_LABEL[table] ?? table} ${Array.isArray(v) ? v.length : v}`
    )
    .join('，')
