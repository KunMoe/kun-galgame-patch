const loaded = new Map<number, Promise<TagTreeNode[] | null>>()

export const useTagDescendants = (tagId: () => number) => {
  const api = useApi()
  const nodes = ref<TagTreeNode[] | null>(null)
  const failed = ref(false)

  const load = (id: number) => {
    const hit = loaded.get(id)
    if (hit) return hit
    const pending = api
      .get<{ items: TagTreeNode[] }>(`/tag/${id}/descendants`)
      .then((res) => {
        if (res.code !== 0) {
          loaded.delete(id)
          return null
        }
        return res.data.items
      })
    loaded.set(id, pending)
    return pending
  }

  watch(
    tagId,
    async (id) => {
      nodes.value = null
      failed.value = false
      const result = await load(id)
      if (id !== tagId()) return
      nodes.value = result
      failed.value = result === null
    },
    { immediate: true }
  )

  return { nodes, failed }
}
