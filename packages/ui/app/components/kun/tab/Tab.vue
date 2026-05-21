<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import {
  kunBgClasses,
  kunTextClasses,
  kunBorderClasses,
} from '../ui/variants'
import type {
  KunTabColor,
  KunTabItem,
  KunTabOrientation,
  KunTabSize,
  KunTabVariant,
} from './type'

const props = withDefaults(
  defineProps<{
    items: KunTabItem[]
    variant?: KunTabVariant
    color?: KunTabColor
    size?: KunTabSize
    orientation?: KunTabOrientation
    fullWidth?: boolean
    disabled?: boolean
    disableAnimation?: boolean
    scrollable?: boolean
    iconSize?: string
    className?: string
    innerClassName?: string
  }>(),
  {
    variant: 'underlined',
    color: 'primary',
    size: 'md',
    orientation: 'horizontal',
    fullWidth: false,
    disabled: false,
    disableAnimation: false,
    scrollable: false,
    iconSize: '1em',
    className: '',
    innerClassName: '',
  }
)

const value = defineModel<string>({ required: true })

const emit = defineEmits<{
  change: [value: string]
}>()

const isVertical = computed(() => props.orientation === 'vertical')

const sizeClasses: Record<KunTabSize, string> = {
  sm: 'text-sm px-2.5 py-1.5',
  md: 'text-sm px-3 py-2',
  lg: 'text-base px-4 py-2.5',
}

const sizeGap: Record<KunTabSize, string> = {
  sm: 'gap-1',
  md: 'gap-1.5',
  lg: 'gap-2',
}

const tabRefs = ref<HTMLElement[]>([])

const setTabRef = (el: Element | null | { $el?: Element } | undefined, idx: number) => {
  const node =
    el && typeof el === 'object' && '$el' in el
      ? ((el as { $el?: Element }).$el ?? null)
      : (el as Element | null)
  if (node instanceof HTMLElement) {
    tabRefs.value[idx] = node
  } else if (el != null && import.meta.dev) {
    console.warn('[KunTab] unexpected ref payload at index', idx, el)
  }
}

const currentIndex = computed(() =>
  props.items.findIndex((i) => i.value === value.value)
)

// Sliding indicator style — only used by underlined/solid/light.
// `bordered` / `pills` render the highlight per-tab and skip the indicator.
//
// underlined: a 2px line on the cross-axis (bottom of the tablist for
//   horizontal, left side for vertical). Width matches the tab's main-axis
//   size so the line is exactly under/beside the active tab.
// solid / light: a full-size panel behind the tab. Width AND height match
//   the tab so the highlight sits exactly over the active tab.
const indicatorStyle = ref<Record<string, string>>({})

const updateIndicator = () => {
  const idx = currentIndex.value
  const el = tabRefs.value[idx]
  if (!el) {
    indicatorStyle.value = {}
    return
  }
  const isLine = props.variant === 'underlined'
  if (isVertical.value) {
    indicatorStyle.value = {
      transform: `translateY(${el.offsetTop}px)`,
      height: `${el.offsetHeight}px`,
      width: isLine ? '2px' : `${el.offsetWidth}px`,
    }
  } else {
    indicatorStyle.value = {
      transform: `translateX(${el.offsetLeft}px)`,
      width: `${el.offsetWidth}px`,
      height: isLine ? '2px' : `${el.offsetHeight}px`,
    }
  }
}

watch(
  [value, () => props.items.length, () => props.orientation],
  () => nextTick(updateIndicator),
  { immediate: true }
)

const focusTab = (idx: number) => {
  const el = tabRefs.value[idx]
  if (el) el.focus()
}

const moveFocus = (delta: number, e?: KeyboardEvent) => {
  e?.preventDefault()
  const enabled = props.items
    .map((it, i) => (it.disabled || props.disabled ? -1 : i))
    .filter((i) => i >= 0)
  if (!enabled.length) return
  const cur = enabled.indexOf(currentIndex.value)
  // When nothing is selected yet (cur < 0), the ARIA pattern says
  // ArrowRight/Down should land on the first item and ArrowLeft/Up on
  // the last. Without this guard, `(-1 + 1 + n) % n` happens to give
  // the right answer for forward but `(-1 - 1 + n) % n` gives n-2,
  // not n-1.
  let nextIdx: number | undefined
  if (cur < 0) {
    nextIdx = enabled[delta > 0 ? 0 : enabled.length - 1]
  } else {
    nextIdx = enabled[(cur + delta + enabled.length) % enabled.length]
  }
  if (nextIdx === undefined) return
  const item = props.items[nextIdx]
  if (item) {
    void selectTab(item, nextIdx)
    focusTab(nextIdx)
  }
}

const onKeydown = (e: KeyboardEvent, idx: number) => {
  if (props.disabled) return
  const forward = isVertical.value ? 'ArrowDown' : 'ArrowRight'
  const backward = isVertical.value ? 'ArrowUp' : 'ArrowLeft'
  switch (e.key) {
    case forward:
      moveFocus(1, e)
      break
    case backward:
      moveFocus(-1, e)
      break
    case 'Home':
      e.preventDefault()
      {
        const first = props.items.findIndex(
          (it) => !(it.disabled || props.disabled)
        )
        if (first >= 0 && props.items[first]) {
          void selectTab(props.items[first], first)
          focusTab(first)
        }
      }
      break
    case 'End':
      e.preventDefault()
      for (let i = props.items.length - 1; i >= 0; i--) {
        const it = props.items[i]
        if (it && !(it.disabled || props.disabled)) {
          void selectTab(it, i)
          focusTab(i)
          break
        }
      }
      break
    case 'Enter':
    case ' ': {
      e.preventDefault()
      const item = props.items[idx]
      if (item) void selectTab(item, idx)
      break
    }
  }
}

const selectTab = async (item: KunTabItem, _idx: number) => {
  if (props.disabled || item.disabled) return
  value.value = item.value
  emit('change', item.value)
  if (item.href) {
    await navigateTo(item.href)
  }
}

const isSelected = (item: KunTabItem) => value.value === item.value

const containerClasses = computed(() =>
  cn(
    'relative',
    isVertical.value ? 'inline-flex flex-col' : 'inline-flex',
    props.fullWidth && 'w-full',
    props.disabled && 'opacity-50 cursor-not-allowed',
    props.scrollable &&
      (isVertical.value
        ? 'max-h-full overflow-y-auto scrollbar-hide'
        : 'max-w-full overflow-x-auto scrollbar-hide'),
    props.className
  )
)

const listClasses = computed(() => {
  const base = isVertical.value
    ? cn('relative flex flex-col items-stretch', sizeGap[props.size])
    : cn('relative flex items-center', sizeGap[props.size])
  switch (props.variant) {
    case 'underlined':
      return cn(
        base,
        isVertical.value
          ? 'border-l border-default-200'
          : 'border-b border-default-200',
        props.innerClassName
      )
    case 'solid':
    case 'light':
      return cn(
        base,
        'border border-default-200 rounded-lg p-1 bg-content2/30',
        props.innerClassName
      )
    case 'bordered':
      return cn(
        base,
        'border border-default-200 rounded-lg p-1',
        props.innerClassName
      )
    case 'pills':
      return cn(base, props.innerClassName)
    default:
      return cn(base, props.innerClassName)
  }
})

// Per-tab classes. Most variants apply a transparent base and the
// indicator <div> draws the active state — except `bordered` which puts
// the active mark on the tab itself.
const tabClasses = (item: KunTabItem) => {
  const selected = isSelected(item)
  const base = cn(
    'relative z-10 inline-flex items-center justify-center cursor-pointer select-none whitespace-nowrap transition-colors',
    sizeClasses[props.size],
    item.disabled && 'opacity-50 cursor-not-allowed',
    isVertical.value && props.fullWidth && 'w-full'
  )
  switch (props.variant) {
    case 'underlined':
      return cn(
        base,
        selected
          ? kunTextClasses[props.color]
          : 'text-default-500 hover:text-foreground'
      )
    case 'solid':
      return cn(
        base,
        'rounded-md',
        selected ? 'text-white' : 'text-default-500 hover:text-foreground'
      )
    case 'light':
      return cn(
        base,
        'rounded-md',
        selected
          ? kunTextClasses[props.color]
          : 'text-default-500 hover:text-foreground'
      )
    case 'bordered':
      return cn(
        base,
        'rounded-md border',
        selected
          ? cn(kunBorderClasses[props.color], kunTextClasses[props.color])
          : 'border-transparent text-default-500 hover:text-foreground'
      )
    case 'pills':
      return cn(
        base,
        'rounded-full',
        selected
          ? cn(kunBgClasses[props.color], 'text-white')
          : 'text-default-500 hover:text-foreground'
      )
    default:
      return base
  }
}

// Sliding indicator class. underlined → 2px line anchored to the
// cross-axis edge. solid / light → full panel behind the active tab,
// positioned solely via inline transform/width/height so we don't
// double-up with utility offsets like `left-1`.
const softBgByColor: Record<KunTabColor, string> = {
  default: 'bg-default/15',
  primary: 'bg-primary/15',
  secondary: 'bg-secondary/15',
  success: 'bg-success/15',
  warning: 'bg-warning/15',
  danger: 'bg-danger/15',
  info: 'bg-info/15',
}

const indicatorClasses = computed(() => {
  switch (props.variant) {
    case 'underlined':
      // 2px line: pin to the cross-axis edge so the inline `top-0` /
      // `bottom-0` controls position; main-axis offset comes from style.
      return cn(
        'absolute rounded-full',
        kunBgClasses[props.color],
        isVertical.value ? 'left-0 top-0' : 'bottom-0 left-0'
      )
    case 'solid':
      // Panel: only `absolute` + rounded; inline style provides
      // translate + width + height so the indicator sits exactly on
      // the tab no matter the parent's padding.
      return cn('absolute top-0 left-0 rounded-md', kunBgClasses[props.color])
    case 'light':
      return cn(
        'absolute top-0 left-0 rounded-md',
        softBgByColor[props.color]
      )
    default:
      return null
  }
})

const showIndicator = computed(
  () =>
    !!indicatorClasses.value &&
    currentIndex.value >= 0 &&
    Object.keys(indicatorStyle.value).length > 0
)

const indicatorMergedStyle = computed(() => {
  const base = { ...indicatorStyle.value }
  if (!props.disableAnimation) {
    base.transition =
      'transform .25s cubic-bezier(.4,0,.2,1), width .25s cubic-bezier(.4,0,.2,1), height .25s cubic-bezier(.4,0,.2,1)'
  }
  return base
})
</script>

<template>
  <div :class="containerClasses">
    <div :class="listClasses" role="tablist" :aria-orientation="orientation">
      <div
        v-if="showIndicator"
        aria-hidden="true"
        :class="indicatorClasses!"
        :style="indicatorMergedStyle"
      />

      <button
        v-for="(item, index) in items"
        :key="item.value"
        :ref="(el) => setTabRef(el as Element | null, index)"
        type="button"
        role="tab"
        :aria-selected="isSelected(item)"
        :aria-disabled="item.disabled || disabled"
        :tabindex="isSelected(item) && !item.disabled && !disabled ? 0 : -1"
        :disabled="item.disabled || disabled"
        :class="tabClasses(item)"
        @click="selectTab(item, index)"
        @keydown="onKeydown($event, index)"
      >
        <KunIcon
          v-if="item.icon"
          :name="item.icon"
          :size="iconSize"
          class="shrink-0"
        />
        <span v-if="item.textValue">{{ item.textValue }}</span>
      </button>
    </div>
  </div>
</template>
