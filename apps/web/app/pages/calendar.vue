<script setup lang="ts">
// Galgame 新作发售月表 — a release calendar backed by the wiki calendar API
// (docs/galgame_wiki/01-galgame.md §发售月历) via moyu's /galgame/calendar
// endpoints. Each entry is stamped has_patch by the backend: entries moyu has a
// patch for link to /patch/:id (badged 本站有补丁); the rest link to the wiki
// entry page. The month + two "未定档" buckets (年内待定 / 发售待定) together cover
// every release so nothing with a fuzzy date is hidden.
//
// Kept alive via the central include list in app.vue, keyed by this name; `month`
// is a computed off ?month=, so reactivation re-reads the URL for the right month.
defineOptions({ name: 'calendar-page' })

useKunSeoMeta({
  title: 'Galgame 发售月表',
  description:
    'Galgame 新作发售月历，按月查看本月发售与即将发售的 Galgame，本站已有补丁的作品可一键进入下载页。'
})

const route = useRoute()
const router = useRouter()
const api = useApi()

// Wiki frontend origin — no-patch entries link to its galgame page.
const config = useRuntimeConfig()
const wikiOrigin =
  ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
  'https://wiki.kungal.com'

// Month in the URL (?month=YYYY-MM) so back-nav / shared links restore it; empty
// → backend resolves to the current JST month and echoes it back.
const month = computed({
  get: () => String(route.query.month ?? ''),
  set: (v: string) =>
    router.push({ query: { ...route.query, month: v || undefined } })
})

const { data, pending } = await useAsyncData<CalendarMonthResponse | null>(
  () => `calendar-${month.value || 'current'}`,
  async () => {
    const qs = month.value ? `?month=${month.value}` : ''
    const res = await api.get<CalendarMonthResponse>(`/galgame/calendar${qs}`)
    return res.code === 0 ? res.data : null
  },
  { watch: [month] }
)

const curMonth = computed(() => data.value?.month ?? '')
const meta = computed(() => data.value?.meta ?? null)
const items = computed<CalendarItem[]>(() => data.value?.items ?? [])

// The two "no firm date" buckets — fetched once, independent of month nav.
const { data: buckets } = await useAsyncData(
  'calendar-buckets',
  async () => {
    const [p, t] = await Promise.all([
      api.get<CalendarBucketResponse>('/galgame/calendar/pending'),
      api.get<CalendarBucketResponse>('/galgame/calendar/tba')
    ])
    return {
      pendingYear: p.code === 0 ? (p.data?.year ?? '') : '',
      pending: p.code === 0 ? (p.data?.items ?? []) : [],
      tba: t.code === 0 ? (t.data?.items ?? []) : []
    }
  },
  { default: () => ({ pendingYear: '', pending: [], tba: [] }) }
)

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']

const monthLabel = computed(() => {
  const [y, mo] = curMonth.value.split('-')
  return y && mo ? `${y} 年 ${Number(mo)} 月` : ''
})

// "6 月 15 日 · 周一" from YYYY-MM-DD (local Date, so no UTC day-shift).
const dayLabel = (ymd: string) => {
  const [y, m, d] = ymd.split('-').map(Number)
  if (!y || !m || !d) return ymd
  return `${m} 月 ${d} 日 · 周${WEEKDAYS[new Date(y, m - 1, d).getDay()]}`
}

// Group day-precision items by exact day (ascending). month-precision entries
// ("日期待定" within the month — release_date YYYY-MM-01 + precision='month')
// collect into a trailing bucket so they don't masquerade as the 1st.
interface DayGroup {
  key: string
  label: string
  items: CalendarItem[]
  isToday: boolean
}
const dayGroups = computed<DayGroup[]>(() => {
  const exact = new Map<string, CalendarItem[]>()
  const tbd: CalendarItem[] = []
  for (const it of items.value) {
    const precision = it.galgame?.release_precision
    const date = it.galgame?.release_date ?? ''
    if (precision === 'month' || !date) {
      tbd.push(it)
      continue
    }
    const day = date.slice(0, 10)
    if (!exact.has(day)) exact.set(day, [])
    exact.get(day)!.push(it)
  }
  const groups: DayGroup[] = [...exact.entries()]
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([key, its]) => ({
      key,
      label: dayLabel(key),
      items: its,
      isToday: key === data.value?.today
    }))
  if (tbd.length) {
    groups.push({
      key: 'tbd',
      label: '本月内 · 具体日期待定',
      items: tbd,
      isToday: false
    })
  }
  return groups
})

const goPrev = () => {
  if (meta.value?.has_prev) month.value = meta.value.prev_month
}
const goNext = () => {
  if (meta.value?.has_next) month.value = meta.value.next_month
}
// "回到本月" — visible only when not already on the wiki's current (JST) month.
const todayMonth = computed(() => data.value?.today?.slice(0, 7) ?? '')
const isCurrentMonth = computed(
  () => !todayMonth.value || curMonth.value === todayMonth.value
)
const goToday = () => {
  month.value = ''
}
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader
      name="Galgame 发售月表"
      description="按月浏览 Galgame 新作发售月历，本站已有补丁的作品可直接进入下载页"
    />

    <!-- Month navigation -->
    <div class="flex items-center justify-center gap-3">
      <KunButton
        variant="flat"
        color="default"
        is-icon-only
        aria-label="上个月"
        :disabled="pending || !meta?.has_prev"
        @click="goPrev"
      >
        <KunIcon name="lucide:chevron-left" class="size-5" />
      </KunButton>

      <div class="flex min-w-40 flex-col items-center">
        <span class="text-lg font-semibold">{{ monthLabel || '加载中' }}</span>
        <button
          v-if="!isCurrentMonth"
          class="text-primary text-xs hover:underline"
          @click="goToday"
        >
          回到本月
        </button>
      </div>

      <KunButton
        variant="flat"
        color="default"
        is-icon-only
        aria-label="下个月"
        :disabled="pending || !meta?.has_next"
        @click="goNext"
      >
        <KunIcon name="lucide:chevron-right" class="size-5" />
      </KunButton>
    </div>

    <KunLoading v-if="pending && !data" description="正在加载发售月历..." />

    <!-- Month grid, grouped by release day -->
    <template v-else>
      <KunNull
        v-if="!dayGroups.length"
        description="本月暂无收录的发售信息"
      />

      <section
        v-for="g in dayGroups"
        :key="g.key"
        class="space-y-3"
      >
        <div class="flex items-center gap-3">
          <div
            class="h-6 w-1 rounded"
            :class="g.isToday ? 'bg-primary' : 'bg-default-300'"
          />
          <h2 class="text-lg font-bold">{{ g.label }}</h2>
          <KunChip v-if="g.isToday" color="primary" variant="flat" size="sm">
            今天
          </KunChip>
          <span class="text-default-400 text-sm">{{ g.items.length }} 部</span>
        </div>
        <div
          class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
        >
          <CalendarCard
            v-for="it in g.items"
            :key="it.id"
            :item="it"
            :wiki-origin="wikiOrigin"
          />
        </div>
      </section>
    </template>

    <!-- "No firm date" buckets: year-only month-TBD + global TBA -->
    <section
      v-if="buckets?.pending.length || buckets?.tba.length"
      class="space-y-6 border-t pt-6"
    >
      <div v-if="buckets.pending.length" class="space-y-3">
        <div class="flex items-center gap-3">
          <div class="bg-warning h-6 w-1 rounded" />
          <h2 class="text-lg font-bold">
            {{ buckets.pendingYear }} 年内发售 · 月份待定
          </h2>
          <span class="text-default-400 text-sm">
            {{ buckets.pending.length }} 部
          </span>
        </div>
        <div
          class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
        >
          <CalendarCard
            v-for="it in buckets.pending"
            :key="it.id"
            :item="it"
            :wiki-origin="wikiOrigin"
          />
        </div>
      </div>

      <div v-if="buckets.tba.length" class="space-y-3">
        <div class="flex items-center gap-3">
          <div class="bg-default-300 h-6 w-1 rounded" />
          <h2 class="text-lg font-bold">发售日期待定（TBA）</h2>
          <span class="text-default-400 text-sm">
            {{ buckets.tba.length }} 部
          </span>
        </div>
        <div
          class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
        >
          <CalendarCard
            v-for="it in buckets.tba"
            :key="it.id"
            :item="it"
            :wiki-origin="wikiOrigin"
          />
        </div>
      </div>
    </section>
  </div>
</template>
