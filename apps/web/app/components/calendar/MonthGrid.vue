<script setup lang="ts">
// Sticky month-at-a-glance grid. Each day with releases is a clickable cell
// (count badge + state color); clicking emits `select` so the page scrolls its
// day-row list to that day. Pure presentation — the page computes `cells`.
interface GridCell {
  day: number // 0 = leading/trailing blank
  key: string // YYYY-MM-DD, '' for a blank
  count: number
  state: 'today' | 'upcoming' | 'past' | 'empty'
}

interface Props {
  cells: GridCell[]
  selected: string
  tbdCount?: number
}

defineProps<Props>()
const emit = defineEmits<{ select: [key: string] }>()

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']
</script>

<template>
  <div>
    <div
      class="text-default-400 mb-1 grid grid-cols-7 gap-1 text-center text-xs"
    >
      <div v-for="w in WEEKDAYS" :key="w">{{ w }}</div>
    </div>

    <div class="grid grid-cols-7 gap-1">
      <template v-for="(c, i) in cells" :key="i">
        <div v-if="c.day === 0" />
        <button
          v-else
          type="button"
          :disabled="c.count === 0"
          class="relative flex min-h-11 items-center justify-center rounded-lg border text-sm transition-colors"
          :class="[
            c.count === 0 ? 'cursor-default' : 'hover:border-primary cursor-pointer',
            selected === c.key
              ? 'border-primary bg-default-100'
              : c.count
                ? 'border-default-200'
                : 'border-transparent',
            c.state === 'today'
              ? 'text-primary font-bold'
              : c.count
                ? 'text-default-600'
                : 'text-default-400'
          ]"
          @click="c.count && emit('select', c.key)"
        >
          {{ c.day }}
          <span
            v-if="c.count"
            class="bg-primary absolute -top-1.5 -right-1.5 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] leading-none font-medium text-white"
          >
            {{ c.count > 99 ? '99+' : c.count }}
          </span>
        </button>
      </template>
    </div>

    <button
      v-if="tbdCount"
      type="button"
      class="mt-1.5 flex w-full items-center justify-center gap-1.5 rounded-lg border py-2 text-xs transition-colors"
      :class="
        selected === 'tbd'
          ? 'border-primary bg-default-100'
          : 'border-default-200 hover:border-primary'
      "
      @click="emit('select', 'tbd')"
    >
      <KunIcon name="lucide:calendar-clock" class="size-4" />
      日期待定 · {{ tbdCount }}
    </button>
  </div>
</template>
