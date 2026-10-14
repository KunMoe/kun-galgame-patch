<script setup lang="ts">
import { KUN_LIBRARY_TABS, KUN_RATING_PLAY_STATE_MAP } from '~/constants/rating'

defineOptions({ name: 'user-library' })

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const userId = computed(() => Number(route.params.id))
const isSelf = computed(() => userStore.user.id === userId.value)

const state = computed(() => {
  const s = String(route.query.state ?? '')
  return KUN_LIBRARY_TABS.some((t) => t.value === s) ? s : 'all'
})

const fetchPage = (cursor = '') => {
  const query = new URLSearchParams()
  if (state.value !== 'all') query.set('state', state.value)
  if (cursor) query.set('cursor', cursor)
  const qs = query.toString()
  return api.get<GalgameLibraryPage>(
    `/user/${userId.value}/library${qs ? `?${qs}` : ''}`
  )
}

const { data, pending } = await useAsyncData(
  () => `user-${userId.value}-library-${state.value}`,
  async () => {
    const res = await fetchPage()
    return res.code === 0 ? res.data : null
  }
)

const items = ref<GalgameLibraryItem[]>([])
const nextCursor = ref('')
const loadingMore = ref(false)

watch(
  data,
  (page) => {
    items.value = page?.items ?? []
    nextCursor.value = page?.next_cursor ?? ''
  },
  { immediate: true }
)

const loadMore = async () => {
  if (loadingMore.value || !nextCursor.value) return
  loadingMore.value = true
  const res = await fetchPage(nextCursor.value)
  loadingMore.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '加载失败，请稍后再试', 'error')
    return
  }
  items.value.push(...res.data.items)
  nextCursor.value = res.data.next_cursor
}

const tabs = computed(() =>
  KUN_LIBRARY_TABS.map((t) => {
    const count =
      t.value === 'all' ? data.value?.total : data.value?.counts?.[t.value]
    return {
      value: t.value,
      textValue: count === undefined ? t.label : `${t.label} ${count}`,
      href: `/user/${userId.value}/library${t.value === 'all' ? '' : `?state=${t.value}`}`
    }
  })
)

const emptyText = computed(() => {
  if (state.value !== 'all') return '这里还没有游戏'
  return isSelf.value
    ? '你还没有标记过游戏，在游戏页面点「评分」就能记下游玩状态'
    : 'TA 还没有标记过游戏'
})

const formatPlaytime = (minutes: number) =>
  minutes < 60
    ? `${minutes} 分钟`
    : `${Math.round((minutes / 60) * 10) / 10} 小时`
</script>

<template>
  <div class="space-y-4">
    <KunNull
      v-if="data?.visibility === 'hidden'"
      description="TA 未公开游玩记录"
    />
    <KunNull
      v-else-if="!data && !pending"
      description="游戏库读取失败，请稍后再试"
    />
    <template v-else>
      <KunInfo
        v-if="data?.visibility === 'owner_only'"
        color="default"
        variant="flat"
        title="仅自己可见"
      >
        <p>
          你的游玩记录没有公开，别人打开这里只会看到「TA 未公开游玩记录」。
          <NuxtLink to="/settings/user" class="text-primary-text underline">
            去设置里公开
          </NuxtLink>
        </p>
      </KunInfo>

      <KunTab
        :model-value="state"
        :items="tabs"
        variant="light"
        color="primary"
        size="sm"
      />

      <KunLoading v-if="pending" description="加载中..." />
      <KunNull v-else-if="!items.length" :description="emptyText" />
      <GalgameList v-else :items="items">
        <template #meta="{ item }">
          <p
            class="text-default-500 mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs"
          >
            <span class="text-primary-text font-medium">
              {{
                KUN_RATING_PLAY_STATE_MAP[item.play_state] ?? item.play_state
              }}
            </span>
            <span v-if="item.minutes" class="flex items-center gap-0.5">
              <KunIcon name="lucide:clock" class="size-3" />
              {{ formatPlaytime(item.minutes) }}
            </span>
            <span
              v-if="item.overall !== null"
              class="flex items-center gap-0.5"
            >
              <KunIcon name="lucide:star" class="size-3" />
              {{ item.overall }}
            </span>
          </p>
        </template>
      </GalgameList>

      <div v-if="nextCursor && !pending" class="flex justify-center">
        <KunButton
          variant="flat"
          color="primary"
          :loading="loadingMore"
          @click="loadMore"
        >
          加载更多
        </KunButton>
      </div>
    </template>
  </div>
</template>
