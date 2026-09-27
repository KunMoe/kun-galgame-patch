<script setup lang="ts">
useKunDisableSeo('关注动态')

const api = useApi()
const userStore = useUserStore()
const { open: openAuthModal } = useAuthModal()
const { markSeen } = useFollowingUnseen()

const PAGE_SIZE = 20

const fetchPage = (cursor = '') => {
  const query = new URLSearchParams({ limit: String(PAGE_SIZE) })
  if (cursor) query.set('cursor', cursor)
  return api.get<FollowingActivityPage>(`/community/following?${query}`)
}

const { data, pending } = await useAsyncData('following-feed', async () => {
  if (!userStore.isLoggedIn) return null
  const res = await fetchPage()
  return res.code === 0 ? res.data : null
})

const groups = ref<FollowingActivityGroup[]>(data.value?.groups ?? [])
const nextCursor = ref(data.value?.next_cursor ?? '')
const loadingMore = ref(false)

watch(data, (page) => {
  groups.value = page?.groups ?? []
  nextCursor.value = page?.next_cursor ?? ''
})

const loadMore = async () => {
  if (loadingMore.value || !nextCursor.value) return
  loadingMore.value = true
  const res = await fetchPage(nextCursor.value)
  loadingMore.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '加载失败，请稍后再试', 'error')
    return
  }
  groups.value.push(...res.data.groups)
  nextCursor.value = res.data.next_cursor
}

const dropGroup = (id: number) => {
  groups.value = groups.value.filter((g) => g.id !== id)
}

onMounted(() => {
  if (data.value?.seen_mark) markSeen(data.value.seen_mark)
})
</script>

<template>
  <div class="container mx-auto my-4 max-w-3xl space-y-4">
    <KunHeader
      name="关注动态"
      description="你关注的人在鲲 Galgame 补丁和 NextMoe 各站发布的新内容"
    />

    <div v-if="!userStore.isLoggedIn" class="space-y-3 text-center">
      <KunNull description="登录后即可查看你关注的人的动态" />
      <KunButton color="primary" @click="openAuthModal()">登录</KunButton>
    </div>

    <KunLoading v-else-if="pending" description="加载中..." />

    <KunNull v-else-if="!data" description="关注动态加载失败，请稍后再试" />

    <div v-else-if="!groups.length" class="space-y-2">
      <KunNull description="你关注的人还没有动态" />
      <p class="text-default-500 text-center text-sm">
        在用户主页点击「关注」，TA 发布的新补丁以及在 NextMoe
        各站的新动态会出现在这里
      </p>
    </div>

    <template v-else>
      <FollowingGroup
        v-for="group in groups"
        :key="group.id"
        :group="group"
        @vanish="dropGroup(group.id)"
      />
      <div class="flex justify-center">
        <KunButton
          v-if="nextCursor"
          variant="light"
          :loading="loadingMore"
          @click="loadMore"
        >
          加载更多
        </KunButton>
        <span v-else class="text-default-400 text-sm">没有更多动态了</span>
      </div>
    </template>
  </div>
</template>
