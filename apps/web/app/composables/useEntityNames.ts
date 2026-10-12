type EntityNameFamily = Extract<SearchEntityFamily, 'company' | 'tag'>

// A filtered list keeps its 会社 and 标签 in the URL as ids so it can be shared,
// and a link opened cold arrives with nothing to draw its chips with until
// catalog answers the names.
export const useEntityNames = () => {
  const api = useApi()
  const named = reactive(new Map<string, SearchEntityItem>())

  const keyOf = (family: string, id: number) => `${family}:${id}`

  const remember = (item: SearchEntityItem) =>
    named.set(keyOf(item.family, item.id), item)

  const itemOf = (family: EntityNameFamily, id: number) =>
    named.get(keyOf(family, id))

  const labelOf = (family: EntityNameFamily, id: number) => {
    const item = itemOf(family, id)
    return item ? getPreferredLanguageText(item.name) : `#${id}`
  }

  const itemsOf = (family: EntityNameFamily, ids: number[]) =>
    ids
      .map((id) => itemOf(family, id))
      .filter((item): item is SearchEntityItem => !!item)

  const resolve = async (wanted: Record<EntityNameFamily, number[]>) => {
    await Promise.all(
      (Object.keys(wanted) as EntityNameFamily[]).map(async (family) => {
        const missing = wanted[family].filter(
          (id) => id > 0 && !named.has(keyOf(family, id))
        )
        if (!missing.length) {
          return
        }
        const res = await api.get<{ items: SearchEntityItem[] }>(
          `/search/entity/resolve?family=${family}&ids=${missing.join(',')}`
        )
        if (res.code === 0) {
          for (const item of res.data?.items ?? []) {
            remember(item)
          }
        }
      })
    )
  }

  return { remember, itemOf, labelOf, itemsOf, resolve }
}
