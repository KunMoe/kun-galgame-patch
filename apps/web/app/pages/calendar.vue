<script setup lang="ts">
// Galgame 新作发售月表 — release calendar over the wiki calendar API. Renders a
// 3-month window [prev, focus, next] (GET /galgame/calendar/window) as a centered
// scroll "wheel": a sticky focus-month grid (badge counts, click-to-scroll) on the
// left, and the prev/focus/next day-list on the right aligned to the focus month's
// start. Each card carries a countdown + inline 收藏 (favoriting lazily records
// a 未收录 game and subscribes to its new-patch notifications). A patch-status
// filter leans into moyu being a patch site. The two 未定档 buckets sit under the
// grid on desktop. The window endpoint refocuses an empty current month onto the
// latest month with releases, so the wheel always centers on real content.
defineOptions({ name: 'calendar-page' })

useKunSeoMeta({
  title: 'Galgame 发售月表',
  description:
    'Galgame 新作发售月历，按月查看本月发售与即将发售的 Galgame，收藏感兴趣的作品，有补丁时第一时间通知你。'
})

const route = useRoute()
const router = useRouter()
const api = useApi()

const month = computed({
  get: () => String(route.query.month ?? ''),
  set: (v: string) =>
    router.push({ query: { ...route.query, month: v || undefined } })
})

const { data, pending } = await useAsyncData<CalendarWindowResponse | null>(
  () => `calendar-window-${month.value || 'current'}`,
  async () => {
    const qs = month.value ? `?month=${month.value}` : ''
    const res = await api.get<CalendarWindowResponse>(
      `/galgame/calendar/window${qs}`
    )
    return res.code === 0 ? res.data : null
  },
  { watch: [month] }
)

const focusMonth = computed(() => data.value?.month ?? '')
const today = computed(() => data.value?.today ?? '')
const meta = computed(() => data.value?.meta ?? null)
const rawMonths = computed<CalendarMonthSection[]>(() => data.value?.months ?? [])

// The two 无定档 buckets — fetched once, independent of the window.
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

// ── Patch-status filter (moyu is a patch site — scope to downloadable).
type PatchFilter = 'all' | 'has' | 'none'
const patchFilter = ref<PatchFilter>('all')
const allItems = computed(() => rawMonths.value.flatMap((m) => m.items))
const hasCount = computed(() => allItems.value.filter((i) => i.has_patch).length)
const filters = computed(() => [
  { key: 'all' as const, label: '全部', count: allItems.value.length },
  { key: 'has' as const, label: '本站有补丁', count: hasCount.value },
  { key: 'none' as const, label: '暂无补丁', count: allItems.value.length - hasCount.value }
])
const months = computed<CalendarMonthSection[]>(() =>
  rawMonths.value.map((m) => ({
    month: m.month,
    items:
      patchFilter.value === 'has'
        ? m.items.filter((i) => i.has_patch)
        : patchFilter.value === 'none'
          ? m.items.filter((i) => !i.has_patch)
          : m.items
  }))
)
const isEmpty = computed(() => months.value.every((m) => m.items.length === 0))
const focusItems = computed(
  () => months.value.find((m) => m.month === focusMonth.value)?.items ?? []
)

// ── Focus-month grid (left): day counts + 待定 bucket count.
const focusDays = computed(() => {
  const byDay = new Map<string, number>()
  let bucket = 0
  for (const it of focusItems.value) {
    const precision = it.galgame?.release_precision
    const date = it.galgame?.release_date ?? ''
    if (precision === 'month' || !date) {
      bucket++
      continue
    }
    const key = date.slice(0, 10)
    byDay.set(key, (byDay.get(key) ?? 0) + 1)
  }
  return { byDay, bucket }
})
const tbdCount = computed(() => focusDays.value.bucket)

interface GridCell {
  day: number
  key: string
  count: number
  state: 'today' | 'upcoming' | 'past' | 'empty'
}
const gridCells = computed<GridCell[]>(() => {
  const [y, m] = focusMonth.value.split('-').map(Number)
  if (!y || !m) return []
  const daysInMonth = new Date(y, m, 0).getDate()
  const firstWeekday = new Date(y, m - 1, 1).getDay()
  const byDay = focusDays.value.byDay
  const cells: GridCell[] = []
  for (let i = 0; i < firstWeekday; i++) {
    cells.push({ day: 0, key: '', count: 0, state: 'empty' })
  }
  for (let d = 1; d <= daysInMonth; d++) {
    const key = `${focusMonth.value}-${String(d).padStart(2, '0')}`
    let state: GridCell['state'] = 'empty'
    if (key === today.value) state = 'today'
    else if (today.value && key > today.value) state = 'upcoming'
    else if (today.value && key < today.value) state = 'past'
    cells.push({ day: d, key, count: byDay.get(key) ?? 0, state })
  }
  return cells
})

// ── Selected day (grid highlight).
const selected = ref('')
watch(
  [gridCells, focusMonth],
  () => {
    const withGames = gridCells.value.filter((c) => c.count > 0)
    if (!withGames.length) {
      selected.value = tbdCount.value ? 'tbd' : ''
      return
    }
    selected.value =
      withGames.find((c) => c.key === today.value)?.key ?? withGames[0]!.key
  },
  { immediate: true }
)

// ── The wheel: center the focus month in the right scroll panel.
const scrollEl = ref<HTMLElement | null>(null)
const isWheel = () => {
  const vp = scrollEl.value
  return !!vp && vp.scrollHeight > vp.clientHeight + 4
}
const alignFocus = (smooth: boolean) => {
  const vp = scrollEl.value
  if (!vp || !isWheel()) return
  const el = vp.querySelector<HTMLElement>('[data-focus-month]')
  if (!el) return
  // Land at the prev→focus boundary: the focus month's header near the top with
  // a small peek of the previous month above it. (Centering a tall focus month
  // would instead land you in the MIDDLE of it.)
  const offset =
    el.getBoundingClientRect().top - vp.getBoundingClientRect().top + vp.scrollTop
  vp.scrollTo({ top: Math.max(0, offset - 64), behavior: smooth ? 'smooth' : 'auto' })
}
const scheduleAlign = (smooth: boolean) => {
  if (!import.meta.client) return
  nextTick(() => requestAnimationFrame(() => alignFocus(smooth)))
}
onMounted(() => scheduleAlign(false))
watch(focusMonth, () => scheduleAlign(true))

// Grid cell click → scroll the wheel (desktop) / page (mobile) to that day.
const scrollToDay = (key: string) => {
  selected.value = key
  if (!import.meta.client) return
  const id = key === 'tbd' ? `${focusMonth.value}-bucket` : key
  const vp = scrollEl.value
  const el = vp?.querySelector<HTMLElement>(`#cal-day-${id}`)
  if (!el) return
  if (vp && isWheel()) {
    const top =
      el.getBoundingClientRect().top - vp.getBoundingClientRect().top + vp.scrollTop - 8
    vp.scrollTo({ top: Math.max(0, top), behavior: 'smooth' })
  } else {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

// ── Month navigation (shifts the window by one month).
const monthLabel = computed(() => {
  const [y, mo] = focusMonth.value.split('-')
  return y && mo ? `${y} 年 ${Number(mo)} 月` : ''
})
const todayMonth = computed(() => today.value.slice(0, 7))
const isCurrentMonth = computed(() => {
  if (!todayMonth.value) return true
  if (focusMonth.value === todayMonth.value) return true
  return !month.value && focusMonth.value === meta.value?.max_month
})
const goPrev = () => {
  if (meta.value?.has_prev) month.value = meta.value.prev_month
}
const goNext = () => {
  if (meta.value?.has_next) month.value = meta.value.next_month
}
const goToday = () => {
  month.value = ''
}
</script>

<template>
  <div class="container mx-auto my-4 space-y-4">
    <KunHeader
      name="Galgame 发售月表"
      description="按月浏览 Galgame 新作发售月历，收藏感兴趣的作品，有补丁时第一时间通知你"
    />

    <div class="flex flex-wrap items-center gap-2">
      <KunButton
        v-for="f in filters"
        :key="f.key"
        :variant="patchFilter === f.key ? 'flat' : 'light'"
        :color="patchFilter === f.key ? 'primary' : 'default'"
        rounded="full"
        size="sm"
        @click="patchFilter = f.key"
      >
        {{ f.label }}
        <span class="text-default-400 ml-1">{{ f.count }}</span>
      </KunButton>
    </div>

    <div class="grid gap-6 lg:grid-cols-3">
      <!-- Left column: nav + grid + 无定档 buckets as ONE sticky unit (desktop). -->
      <aside class="lg:col-span-1">
        <div
          class="space-y-4 lg:sticky lg:top-20 lg:max-h-[calc(100dvh-7rem)] lg:overflow-y-auto"
        >
          <div class="flex items-center justify-between gap-2">
            <KunButton
              variant="flat"
              color="default"
              is-icon-only
              size="sm"
              aria-label="上个月"
              :disabled="pending || !meta?.has_prev"
              @click="goPrev"
            >
              <KunIcon name="lucide:chevron-left" class="size-5" />
            </KunButton>

            <div class="flex flex-col items-center">
              <span class="font-semibold">{{ monthLabel || '加载中' }}</span>
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
              size="sm"
              aria-label="下个月"
              :disabled="pending || !meta?.has_next"
              @click="goNext"
            >
              <KunIcon name="lucide:chevron-right" class="size-5" />
            </KunButton>
          </div>

          <div class="border-default/20 bg-content1 rounded-2xl border p-3">
            <CalendarMonthGrid
              :cells="gridCells"
              :selected="selected"
              :tbd-count="tbdCount"
              @select="scrollToDay"
            />
          </div>

          <!-- 无定档 buckets — desktop only. -->
          <div
            v-if="buckets?.pending.length || buckets?.tba.length"
            class="hidden space-y-5 lg:block"
          >
            <div v-if="buckets.pending.length" class="space-y-2">
              <div class="flex items-center gap-2">
                <div class="bg-warning h-5 w-1 rounded" />
                <h3 class="text-sm font-bold">
                  {{ buckets.pendingYear }} 年内 · 月份待定
                </h3>
                <span class="text-default-400 text-xs">
                  {{ buckets.pending.length }}
                </span>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <CalendarCard
                  v-for="it in buckets.pending"
                  :key="it.id"
                  :item="it"
                  :today="today"
                />
              </div>
            </div>

            <div v-if="buckets.tba.length" class="space-y-2">
              <div class="flex items-center gap-2">
                <div class="bg-default-300 h-5 w-1 rounded" />
                <h3 class="text-sm font-bold">发售日期待定</h3>
                <span class="text-default-400 text-xs">
                  {{ buckets.tba.length }}
                </span>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <CalendarCard
                  v-for="it in buckets.tba"
                  :key="it.id"
                  :item="it"
                  :today="today"
                />
              </div>
            </div>
          </div>
        </div>
      </aside>

      <!-- Right column: the 3-month wheel. -->
      <div class="lg:col-span-2">
        <KunLoading v-if="pending && !data" description="正在加载发售月历..." />

        <div
          v-else
          ref="scrollEl"
          class="lg:sticky lg:top-20 lg:h-[calc(100dvh-7rem)] lg:overflow-y-auto lg:pr-1"
        >
          <KunNull
            v-if="isEmpty"
            :description="
              patchFilter === 'has'
                ? '附近月份本站暂无有补丁的作品'
                : patchFilter === 'none'
                  ? '附近月份的作品本站均已有补丁'
                  : '附近暂无收录的发售信息'
            "
          />
          <CalendarMonthList
            v-else
            :months="months"
            :today="today"
            :focus-month="focusMonth"
          />
        </div>
      </div>
    </div>
  </div>
</template>
