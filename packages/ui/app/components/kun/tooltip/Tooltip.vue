<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  useFloating,
  autoUpdate,
  offset,
  flip,
  shift,
  arrow,
  type Placement,
} from '@floating-ui/vue'

interface Props {
  text?: string
  position?: 'top' | 'bottom' | 'left' | 'right'
  className?: string
  // Hover delay-in / delay-out (ms). delay-in prevents flicker on
  // fast pointer travel; delay-out gives time to move into the tooltip
  // body if interactive content is in the slot.
  delayShow?: number
  delayHide?: number
  // Hide entirely below the sm breakpoint — tooltips on mobile are
  // usually noise. Set to false to keep them.
  hideOnMobile?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  text: '',
  position: 'top',
  className: '',
  delayShow: 100,
  delayHide: 0,
  hideOnMobile: true,
})

const triggerRef = ref<HTMLElement | null>(null)
const tooltipRef = ref<HTMLElement | null>(null)
const arrowRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)

let showTimer: ReturnType<typeof setTimeout> | null = null
let hideTimer: ReturnType<typeof setTimeout> | null = null

const placement = computed<Placement>(() => props.position)

const { floatingStyles, middlewareData, placement: actualPlacement } = useFloating(
  triggerRef,
  tooltipRef,
  {
    placement,
    open: isVisible,
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(8),
      flip(),
      shift({ padding: 8 }),
      arrow({ element: arrowRef, padding: 4 }),
    ],
  }
)

// Arrow sits on the opposite side of the tooltip's placement axis.
const arrowStyles = computed(() => {
  const data = middlewareData.value.arrow
  if (!data) return {}
  const side = actualPlacement.value.split('-')[0] as
    | 'top'
    | 'bottom'
    | 'left'
    | 'right'
  const staticSide = {
    top: 'bottom',
    bottom: 'top',
    left: 'right',
    right: 'left',
  }[side]
  return {
    left: data.x != null ? `${data.x}px` : '',
    top: data.y != null ? `${data.y}px` : '',
    [staticSide]: '-4px',
  }
})

const clearTimers = () => {
  if (showTimer) {
    clearTimeout(showTimer)
    showTimer = null
  }
  if (hideTimer) {
    clearTimeout(hideTimer)
    hideTimer = null
  }
}

const show = () => {
  clearTimers()
  if (props.delayShow > 0) {
    showTimer = setTimeout(() => (isVisible.value = true), props.delayShow)
  } else {
    isVisible.value = true
  }
}

const hide = () => {
  clearTimers()
  if (props.delayHide > 0) {
    hideTimer = setTimeout(() => (isVisible.value = false), props.delayHide)
  } else {
    isVisible.value = false
  }
}
</script>

<template>
  <div
    ref="triggerRef"
    :class="cn('relative inline-block', className)"
    @mouseenter="show"
    @mouseleave="hide"
    @focusin="show"
    @focusout="hide"
  >
    <slot />

    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-150 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-100 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="isVisible"
          ref="tooltipRef"
          role="tooltip"
          :class="
            cn(
              'bg-content1 border-default-200 z-50 rounded-lg border px-3 py-2 text-sm font-medium whitespace-nowrap shadow-md',
              hideOnMobile && 'hidden sm:block'
            )
          "
          :style="floatingStyles"
        >
          <slot name="content">{{ text }}</slot>
          <div
            ref="arrowRef"
            class="bg-content1 border-default-200 absolute h-2 w-2 rotate-45 border-r border-b"
            :style="arrowStyles"
          />
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
