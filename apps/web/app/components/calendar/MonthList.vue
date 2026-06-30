<script setup lang="ts">
// The 3-month window's day list (prev / focus / next), rendered as month
// sections so a sparse focus month still shows neighbouring releases. The focus
// section carries [data-focus-month] so the page can scroll-center it (the
// "wheel"). Each day row's tile shows the month-short too, since the section
// header scrolls out of view as you move through the window. Row ids are
// `cal-day-YYYY-MM-DD` (and `-bucket`) so the grid can click-to-scroll.
interface Props {
  months: CalendarMonthSection[]
  today: string
  focusMonth: string
}

const props = defineProps<Props>()

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']

interface DayRow {
  id: string
  day: number
  monthShort: string
  weekday: string
  status: 'past' | 'today' | 'future'
  isBucket: boolean
  items: CalendarItem[]
}
interface Section {
  month: string
  label: string
  isFocus: boolean
  count: number
  rows: DayRow[]
}

const sections = computed<Section[]>(() =>
  props.months.map((m) => {
    const [y, mo] = m.month.split('-').map(Number)
    const monthShort = `${mo} 月`
    const exact = new Map<string, CalendarItem[]>()
    const bucket: CalendarItem[] = []
    for (const it of m.items) {
      const precision = it.galgame?.release_precision
      const date = it.galgame?.release_date ?? ''
      if (precision === 'month' || !date) {
        bucket.push(it)
        continue
      }
      const day = date.slice(0, 10)
      if (!exact.has(day)) exact.set(day, [])
      exact.get(day)!.push(it)
    }
    const rows: DayRow[] = [...exact.entries()]
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([key, items]) => {
        const d = Number(key.slice(8, 10))
        return {
          id: key,
          day: d,
          monthShort,
          weekday: `周${WEEKDAYS[new Date(y!, mo! - 1, d).getDay()]}`,
          status:
            key === props.today ? 'today' : key < props.today ? 'past' : 'future',
          isBucket: false,
          items
        }
      })
    if (bucket.length) {
      rows.push({
        id: `${m.month}-bucket`,
        day: 0,
        monthShort,
        weekday: '日期待定',
        status: 'future',
        isBucket: true,
        items: bucket
      })
    }
    return {
      month: m.month,
      label: `${y} 年 ${mo} 月`,
      isFocus: m.month === props.focusMonth,
      count: m.items.length,
      rows
    }
  })
)

const tileClass = (r: DayRow) => {
  if (r.status === 'today') return 'bg-primary text-white'
  if (r.isBucket) return 'bg-default-100 text-default-400'
  if (r.status === 'future') return 'border-primary text-primary border-2'
  return 'bg-default-100 text-default-500'
}
</script>

<template>
  <div class="space-y-7">
    <section
      v-for="sec in sections"
      :key="sec.month"
      :data-focus-month="sec.isFocus ? sec.month : undefined"
      class="space-y-3"
    >
      <div
        class="bg-background/80 sticky top-0 z-10 flex items-center gap-2 py-1 backdrop-blur"
      >
        <h2
          class="text-base font-bold"
          :class="sec.isFocus ? 'text-primary' : 'text-default-600'"
        >
          {{ sec.label }}
        </h2>
        <KunChip v-if="sec.isFocus" color="primary" variant="flat" size="sm">
          本月
        </KunChip>
        <span class="text-default-400 text-sm">{{ sec.count }} 部</span>
      </div>

      <p v-if="!sec.rows.length" class="text-default-400 py-3 text-sm">
        本月暂无发售的 Galgame
      </p>

      <section
        v-for="r in sec.rows"
        :id="`cal-day-${r.id}`"
        :key="r.id"
        class="border-default/15 flex scroll-mt-4 flex-col gap-3 border-t py-4 first:border-t-0 sm:flex-row sm:gap-5"
      >
        <div
          class="flex shrink-0 items-center gap-3 sm:w-14 sm:flex-col sm:gap-1.5"
        >
          <div
            class="flex size-14 shrink-0 flex-col items-center justify-center rounded-xl"
            :class="tileClass(r)"
          >
            <template v-if="!r.isBucket">
              <span class="text-[10px] leading-none opacity-80">
                {{ r.monthShort }}
              </span>
              <span class="text-2xl leading-none font-bold">{{ r.day }}</span>
            </template>
            <KunIcon v-else name="lucide:calendar-clock" class="size-6" />
          </div>
          <div class="flex items-center gap-2 sm:flex-col sm:gap-1">
            <span class="text-default-500 text-xs">{{ r.weekday }}</span>
            <KunChip
              v-if="r.status === 'today'"
              color="primary"
              variant="solid"
              size="sm"
            >
              今日
            </KunChip>
          </div>
        </div>

        <div class="grid flex-1 grid-cols-2 gap-3 sm:grid-cols-3">
          <CalendarCard
            v-for="it in r.items"
            :key="it.id"
            :item="it"
            :today="today"
          />
        </div>
      </section>
    </section>
  </div>
</template>
