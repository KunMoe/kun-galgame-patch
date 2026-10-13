// The record modal opens from the page header and from the rating tab; a save
// from either has to refresh the tab's list.
export const useRatingRefresh = (workId: MaybeRefOrGetter<number>) => {
  const versions = useState<Record<number, number>>(
    'rating-refresh',
    () => ({})
  )
  const version = computed(() => versions.value[toValue(workId)] ?? 0)
  const bump = () => {
    const id = toValue(workId)
    versions.value = { ...versions.value, [id]: (versions.value[id] ?? 0) + 1 }
  }
  return { version, bump }
}
