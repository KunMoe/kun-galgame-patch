<script setup lang="ts">
import { ref, watch } from 'vue'
import { useEventListener, onClickOutside } from '@vueuse/core'
import {
  useFloating,
  autoUpdate,
  offset,
  flip,
  shift,
  type Placement,
} from '@floating-ui/vue'

type PopoverPosition = 'top-start' | 'top-end' | 'bottom-start' | 'bottom-end'

const props = withDefaults(
  defineProps<{
    position?: PopoverPosition
    innerClass?: string
    autoPosition?: boolean
  }>(),
  {
    position: 'bottom-start',
    innerClass: '',
    autoPosition: false,
  }
)

// @floating-ui placements use the same string format we already accept,
// so the prop maps 1:1 onto its `placement` config.
const isOpen = ref(false)
const triggerRef = ref<HTMLElement | null>(null)
const popoverRef = ref<HTMLElement | null>(null)
const popoverId = `kun-popover-${useId()}`

const { floatingStyles } = useFloating(triggerRef, popoverRef, {
  placement: props.position as Placement,
  open: isOpen,
  // Re-run computation on scroll / resize / element-size changes. Only
  // attached while open to avoid leaking listeners.
  whileElementsMounted: autoUpdate,
  middleware: [
    offset(8),
    // autoPosition=false → respect the literal `position` prop;
    // autoPosition=true → flip + shift into viewport when constrained.
    ...(props.autoPosition ? [flip(), shift({ padding: 8 })] : []),
  ],
})

const toggle = () => {
  isOpen.value = !isOpen.value
}

// Close on outside click — narrowed to the trigger / popover pair via
// onClickOutside (vs. the legacy implementation's global document
// listener that fired for every popover instance on the page).
onClickOutside(triggerRef, (event) => {
  if (popoverRef.value?.contains(event.target as Node)) return
  isOpen.value = false
})

useEventListener('keydown', (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isOpen.value) isOpen.value = false
})

// Expose imperative API for parents who need to close from outside.
defineExpose({
  open: () => (isOpen.value = true),
  close: () => (isOpen.value = false),
  toggle,
})
</script>

<template>
  <div class="relative inline-block">
    <div
      ref="triggerRef"
      @click="toggle"
      @keydown.enter="toggle"
      @keydown.space.prevent="toggle"
      tabindex="0"
      role="button"
      aria-label="popover-trigger"
      :aria-expanded="isOpen"
      :aria-controls="popoverId"
    >
      <slot name="trigger" />
    </div>

    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="transform scale-95 opacity-0"
        enter-to-class="transform scale-100 opacity-100"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="transform scale-100 opacity-100"
        leave-to-class="transform scale-95 opacity-0"
      >
        <div
          v-if="isOpen"
          ref="popoverRef"
          :id="popoverId"
          role="dialog"
          :aria-hidden="!isOpen"
          :class="
            cn(
              'bg-content1 border-default-200 z-50 rounded-lg border shadow-lg',
              innerClass
            )
          "
          :style="floatingStyles"
          @click.stop
        >
          <slot />
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
