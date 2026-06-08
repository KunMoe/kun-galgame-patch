<script setup lang="ts">
// 萌萌点记录 — the user's OWN moemoepoint ledger. OAuth is the source of truth;
// moyu proxies the reduced s2s view via GET /user/moemoepoint/log (id from the
// session, never a path param). Cursor paginated by the last row's id.
const open = defineModel<boolean>({ required: true })

const api = useApi()
const userStore = useUserStore()

interface MoemoepointLogEntry {
  id: number
  delta: number
  reason: string
  source_app: string
  ref: string
  created_at: string
}

const LIMIT = 20

const items = ref<MoemoepointLogEntry[]>([])
const hasMore = ref(false)
const loading = ref(false)
const loadingMore = ref(false)
const loaded = ref(false)
const failed = ref(false)

// reason → human label + icon + color. Mirrors OAuth's reason enum
// (06-moemoepoint.md); admin_*/migration only appear for cross-channel rows.
const REASONS: Record<string, { label: string; icon: string; class: string }> = {
  content_approved: {
    label: '内容被采纳',
    icon: 'lucide:badge-check',
    class: 'text-success-500'
  },
  content_removed: {
    label: '内容被移除',
    icon: 'lucide:badge-x',
    class: 'text-danger-500'
  },
  daily_checkin: {
    label: '每日签到',
    icon: 'lucide:calendar-check',
    class: 'text-primary-500'
  },
  liked: { label: '获得喜欢', icon: 'lucide:heart', class: 'text-danger-500' },
  admin_grant: { label: '管理员发放', icon: 'lucide:gift', class: 'text-success-500' },
  admin_deduct: { label: '管理员扣除', icon: 'lucide:gavel', class: 'text-warning-500' },
  migration: { label: '初始迁移', icon: 'lucide:database', class: 'text-default-400' }
}

const reasonMeta = (reason: string) =>
  REASONS[reason] ?? {
    label: reason,
    icon: 'lucide:circle-dot',
    class: 'text-default-400'
  }

// ref is "type:id" (e.g. "resource:42", "galgame:1207"). Map the ones that have
// an in-site page to a link; everything else (comment, admin:*, …) has no link.
const refLink = (refStr: string): string | null => {
  const [type, id] = (refStr || '').split(':')
  if (!id) return null
  if (type === 'resource') return `/resource/${id}`
  if (type === 'galgame' || type === 'patch') return `/patch/${id}`
  return null
}

const fetchPage = async (beforeID?: number) => {
  const params = new URLSearchParams({ limit: String(LIMIT) })
  if (beforeID) params.set('before_id', String(beforeID))
  const res = await api.get<{ items: MoemoepointLogEntry[]; has_more: boolean }>(
    `/user/moemoepoint/log?${params.toString()}`
  )
  if (res.code !== 0) throw new Error(res.message)
  return res.data
}

const load = async () => {
  loading.value = true
  failed.value = false
  try {
    const data = await fetchPage()
    items.value = data.items ?? []
    hasMore.value = data.has_more
    loaded.value = true
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

const loadMore = async () => {
  const last = items.value[items.value.length - 1]
  if (loadingMore.value || !hasMore.value || !last) return
  loadingMore.value = true
  try {
    const data = await fetchPage(last.id)
    items.value.push(...(data.items ?? []))
    hasMore.value = data.has_more
  } catch {
    useKunMessage('加载更多失败', 'error')
  } finally {
    loadingMore.value = false
  }
}

// Refetch on each open so a record earned this session (e.g. a fresh check-in)
// shows without a page reload.
watch(open, (v) => {
  if (v) load()
})
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-lg w-full border-default-200">
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="flex items-center gap-2 text-lg font-semibold">
          <KunIcon name="lucide:lollipop" class="size-5" />
          萌萌点记录
        </h3>
        <span class="text-foreground/60 text-sm">
          当前 {{ userStore.user.moemoepoint }}
        </span>
      </div>

      <KunLoading v-if="loading" description="加载记录中..." />

      <KunNull v-else-if="failed" description="加载失败, 请稍后再试" />

      <KunNull
        v-else-if="loaded && !items.length"
        description="还没有萌萌点记录哦"
      />

      <ul v-else class="max-h-[60vh] space-y-1 overflow-y-auto">
        <li
          v-for="item in items"
          :key="item.id"
          class="hover:bg-default-100 flex items-center gap-3 rounded-lg px-2 py-2"
        >
          <span
            class="bg-default-100 flex size-9 shrink-0 items-center justify-center rounded-full"
            :class="reasonMeta(item.reason).class"
          >
            <KunIcon :name="reasonMeta(item.reason).icon" class="size-4" />
          </span>
          <div class="min-w-0 flex-1">
            <p class="flex items-center gap-2 text-sm font-medium">
              <span class="truncate">{{ reasonMeta(item.reason).label }}</span>
              <NuxtLink
                v-if="refLink(item.ref)"
                :to="refLink(item.ref)!"
                class="text-primary-500 shrink-0 text-xs hover:underline"
                @click="open = false"
              >
                查看
              </NuxtLink>
            </p>
            <p class="text-foreground/50 text-xs">
              {{ formatTimeDifference(item.created_at) }}
            </p>
          </div>
          <span
            class="shrink-0 text-sm font-semibold tabular-nums"
            :class="item.delta >= 0 ? 'text-success-500' : 'text-danger-500'"
          >
            {{ item.delta >= 0 ? '+' : '' }}{{ item.delta }}
          </span>
        </li>

        <li v-if="hasMore" class="pt-1">
          <KunButton
            variant="light"
            full-width
            :loading="loadingMore"
            :disabled="loadingMore"
            @click="loadMore"
          >
            加载更多
          </KunButton>
        </li>
      </ul>
    </div>
  </KunModal>
</template>
