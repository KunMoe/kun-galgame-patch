<script setup lang="ts">
defineOptions({ name: 'calendar-page' })

useKunSeoMeta({
  title: 'Galgame 发售月表',
  description:
    'Galgame 新作发售月历，按月查看本月发售与即将发售的 Galgame，收藏感兴趣的作品，有补丁时第一时间通知你。'
})

const route = useRoute()
const router = useRouter()
const api = useApi()
const { blockKey } = useTagBlocks()

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
    let d = res.code === 0 ? res.data : null
    if (
      d &&
      !month.value &&
      d.items.length === 0 &&
      d.meta?.max_month &&
      d.meta.max_month !== d.month
    ) {
      const r2 = await api.get<CalendarMonthResponse>(
        `/galgame/calendar?month=${d.meta.max_month}`
      )
      if (r2.code === 0 && r2.data) d = r2.data
    }
    return d
  },
  { watch: [month, blockKey] }
)

const curMonth = computed(() => data.value?.month ?? '')
const meta = computed(() => data.value?.meta ?? null)
const today = computed(() => data.value?.today ?? '')
const allItems = computed<CalendarItem[]>(() => data.value?.items ?? [])

type PatchFilter = 'all' | 'has' | 'none'
const patchFilter = ref<PatchFilter>('all')
const hasCount = computed(() => allItems.value.filter((i) => i.has_patch).length)
const filters = computed(() => [
  { key: 'all' as const, label: '全部', count: allItems.value.length },
  { key: 'has' as const, label: '本站有补丁', count: hasCount.value },
  { key: 'none' as const, label: '暂无补丁', count: allItems.value.length - hasCount.value }
])
const items = computed<CalendarItem[]>(() => {
  if (patchFilter.value === 'has') return allItems.value.filter((i) => i.has_patch)
  if (patchFilter.value === 'none') return allItems.value.filter((i) => !i.has_patch)
  return allItems.value
})

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']

interface DayGroup {
  key: string
  day: number
  weekday: string
  isToday: boolean
  isTbd: boolean
  items: CalendarItem[]
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
    .map(([key, its]) => {
      const [y, m, d] = key.split('-').map(Number)
      return {
        key,
        day: d!,
        isTbd: false,
        isToday: key === today.value,
        weekday: `周${WEEKDAYS[new Date(y!, m! - 1, d).getDay()]}`,
        items: its
      }
    })
  if (tbd.length) {
    groups.push({ key: 'tbd', day: 0, weekday: '日期待定', isToday: false, isTbd: true, items: tbd })
  }
  return groups
})

interface GridCell {
  day: number
  key: string
  count: number
  state: 'today' | 'upcoming' | 'past' | 'empty'
}
const gridCells = computed<GridCell[]>(() => {
  const [y, m] = curMonth.value.split('-').map(Number)
  if (!y || !m) return []
  const daysInMonth = new Date(y, m, 0).getDate()
  const firstWeekday = new Date(y, m - 1, 1).getDay()
  const countByDay = new Map<string, number>()
  for (const g of dayGroups.value) {
    if (!g.isTbd) countByDay.set(g.key, g.items.length)
  }
  const cells: GridCell[] = []
  for (let i = 0; i < firstWeekday; i++) {
    cells.push({ day: 0, key: '', count: 0, state: 'empty' })
  }
  for (let d = 1; d <= daysInMonth; d++) {
    const key = `${curMonth.value}-${String(d).padStart(2, '0')}`
    let state: GridCell['state'] = 'empty'
    if (key === today.value) state = 'today'
    else if (today.value && key > today.value) state = 'upcoming'
    else if (today.value && key < today.value) state = 'past'
    cells.push({ day: d, key, count: countByDay.get(key) ?? 0, state })
  }
  return cells
})
const tbdCount = computed(
  () => dayGroups.value.find((g) => g.isTbd)?.items.length ?? 0
)

const selected = ref('')
watch(
  dayGroups,
  (groups) => {
    if (!groups.length) {
      selected.value = ''
      return
    }
    selected.value = groups.some((g) => g.key === today.value)
      ? today.value
      : groups[0]!.key
  },
  { immediate: true }
)
const scrollToDay = (key: string) => {
  selected.value = key
  if (import.meta.client) {
    document
      .getElementById(`cal-day-${key}`)
      ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

const ymd = (s: string) => {
  const [y, m, d] = s.split('-').map(Number)
  return y && m && d ? Date.UTC(y, m - 1, d) : NaN
}

// Every card in a group shares the group's date, so the countdown belongs to the
// day header — on the card it was the same sentence repeated once per poster.
const dayCountdown = (g: DayGroup) => {
  if (g.isTbd || g.isToday || !today.value) return ''
  const days = Math.round((ymd(g.key) - ymd(today.value)) / 86400000)
  if (Number.isNaN(days)) return ''
  return days > 0 ? `还有 ${days} 天` : '已发售'
}

const tileClass = (g: DayGroup) => {
  if (g.isToday) return 'bg-primary text-white'
  if (g.isTbd) return 'bg-default-100 text-default-400'
  if (today.value && g.key > today.value) return 'border-primary text-primary border-2'
  return 'bg-default-100 text-default-500'
}

const monthLabel = computed(() => {
  const [y, mo] = curMonth.value.split('-')
  return y && mo ? `${y} 年 ${Number(mo)} 月` : ''
})
const todayMonth = computed(() => today.value.slice(0, 7))
const isCurrentMonth = computed(() => {
  if (!todayMonth.value) return true
  if (curMonth.value === todayMonth.value) return true
  return !month.value && curMonth.value === meta.value?.max_month
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
  <div class="container mx-auto my-4 space-y-5">
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
      <aside class="lg:col-span-1">
        <div class="space-y-4 lg:sticky lg:top-20">
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
        </div>
      </aside>

      <div class="lg:col-span-2">
        <KunLoading v-if="pending && !data" description="正在加载发售月历..." />

        <KunNull
          v-else-if="!dayGroups.length"
          :description="
            patchFilter === 'has'
              ? '本月本站暂无有补丁的作品'
              : patchFilter === 'none'
                ? '本月作品本站均已有补丁'
                : '本月暂无收录的发售信息'
          "
        />

        <template v-else>
          <section
            v-for="g in dayGroups"
            :id="`cal-day-${g.key}`"
            :key="g.key"
            class="border-default/15 flex scroll-mt-24 flex-col gap-3 border-t py-4 first:border-t-0 sm:flex-row sm:gap-5"
          >
            <div
              class="flex shrink-0 items-center gap-3 sm:w-14 sm:flex-col sm:gap-1.5"
            >
              <div
                class="flex size-14 shrink-0 flex-col items-center justify-center rounded-xl"
                :class="tileClass(g)"
              >
                <span v-if="!g.isTbd" class="text-2xl leading-none font-bold">
                  {{ g.day }}
                </span>
                <KunIcon v-else name="lucide:calendar-clock" class="size-6" />
              </div>
              <div class="flex items-center gap-2 sm:flex-col sm:gap-1">
                <span class="text-default-500 text-xs">{{ g.weekday }}</span>
                <KunChip
                  v-if="g.isToday"
                  color="primary"
                  variant="solid"
                  size="sm"
                >
                  今日
                </KunChip>
                <span
                  v-else-if="dayCountdown(g)"
                  class="text-default-400 text-xs whitespace-nowrap"
                >
                  {{ dayCountdown(g) }}
                </span>
              </div>
            </div>

            <GalgameCardGrid class="flex-1">
              <CalendarCard v-for="it in g.items" :key="it.id" :item="it" />
            </GalgameCardGrid>
          </section>
        </template>
      </div>
    </div>
  </div>
</template>
