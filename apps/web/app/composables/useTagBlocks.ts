import { KUN_TAG_BLOCKS_MAX } from '~/constants/tagBlock'

// A signed-in reader's list belongs to the NextMoe account (catalog's
// /v2/me/blocked-tags); the cookie only mirrors it so SSR can filter the first
// paint.
export const useTagBlocks = () => {
  const store = useTagBlockStore()
  const userStore = useUserStore()
  const api = useApi()

  const uid = computed(() => userStore.user.id)
  const isSignedIn = computed(() => uid.value > 0)
  const isAccountLoaded = computed(
    () => isSignedIn.value && store.accountUid === uid.value
  )

  const blocks = computed(() => resolveTagBlocks(store.$state, uid.value))
  const count = computed(
    () => blocks.value.tree.length + blocks.value.exact.length
  )
  const blockKey = computed(() => tagBlockQuery(blocks.value))

  // 40399 already raises useApi's own "log in again" toast.
  const fail = (code: number, message: string) => {
    if (code !== 40399) useKunMessage(message, 'error')
    return false
  }

  const modeOf = (id: number): KunTagBlockMode | null => {
    if (blocks.value.tree.includes(id)) return 'tree'
    return blocks.value.exact.includes(id) ? 'exact' : null
  }

  const refresh = async () => {
    const owner = uid.value
    if (owner <= 0) return false
    const res = await api.get<{ blocks: TagBlock[] }>('/tag-block')
    if (res.code !== 0 || uid.value !== owner) return false
    const next = emptyTagBlocks()
    for (const b of res.data.blocks) next[b.mode].push(b.tag_id)
    store.$patch({ account: next, accountUid: owner })
    return true
  }

  const write = (next: KunTagBlockSet) => {
    if (isSignedIn.value) {
      store.$patch({ account: next, accountUid: uid.value })
    } else {
      store.anonymous = next
    }
  }

  const block = async (id: number, mode: KunTagBlockMode = 'tree') => {
    if (isSignedIn.value && !isAccountLoaded.value && !(await refresh())) {
      return fail(0, '读取屏蔽列表失败，请稍后再试')
    }
    if (!modeOf(id) && count.value >= KUN_TAG_BLOCKS_MAX) {
      useKunMessage(`最多屏蔽 ${KUN_TAG_BLOCKS_MAX} 个标签`, 'warn')
      return false
    }
    if (isSignedIn.value) {
      const res = await api.put<TagBlock>(`/tag-block/${id}`, { mode })
      if (res.code !== 0) {
        return fail(res.code, res.message || '屏蔽失败，请稍后再试')
      }
    }
    write(withTagBlockMode(blocks.value, id, mode))
    return true
  }

  const unblock = async (id: number) => {
    if (isSignedIn.value) {
      if (!isAccountLoaded.value && !(await refresh())) {
        return fail(0, '读取屏蔽列表失败，请稍后再试')
      }
      const res = await api.delete(`/tag-block/${id}`)
      if (res.code !== 0) {
        return fail(res.code, res.message || '取消屏蔽失败，请稍后再试')
      }
    }
    write(withTagBlockMode(blocks.value, id, null))
    return true
  }

  return {
    isSignedIn,
    blocks,
    count,
    blockKey,
    modeOf,
    refresh,
    block,
    unblock
  }
}
