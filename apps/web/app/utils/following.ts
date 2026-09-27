const VERB_PHRASE: Record<string, { verb: string; unit: string }> = {
  publish: { verb: '发布了', unit: '个' },
  reply: { verb: '发表了', unit: '条' },
  comment: { verb: '发表了', unit: '条' },
  rate: { verb: '发表了', unit: '条' },
  like: { verb: '推荐了', unit: '个' },
  edit: { verb: '编辑了', unit: '个' }
}

const SITE_LABEL: Record<string, string> = {
  kungal: '鲲 Galgame 论坛',
  letmoe: 'letmoe.com'
}

export const FOLLOWING_OWN_SITE = 'moyu'

const spaced = (label: string) =>
  /^[A-Za-z0-9]/.test(label) ? ` ${label}` : label

export const followingGroupPhrase = (
  group: Pick<FollowingActivityGroup, 'verb' | 'item_count' | 'object_label'>
) => {
  const { verb, unit } = VERB_PHRASE[group.verb] ?? VERB_PHRASE.publish!
  const count = group.item_count > 1 ? ` ${group.item_count} ${unit}` : ''
  return `${verb}${count}${spaced(group.object_label || '内容')}`
}

export const followingSiteLabel = (site: string) =>
  site === FOLLOWING_OWN_SITE ? '' : (SITE_LABEL[site] ?? site)
