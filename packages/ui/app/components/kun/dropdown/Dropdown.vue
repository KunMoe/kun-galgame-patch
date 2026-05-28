<script setup lang="ts">
import { computed, ref, nextTick } from 'vue'
import { onClickOutside, useEventListener } from '@vueuse/core'
import {
  useFloating,
  autoUpdate,
  offset,
  flip,
  shift,
  type Placement,
} from '@floating-ui/vue'
import { kunVariantClasses } from '../ui/variants'
import type { KunUIColor } from '../ui/type'
import type { KunDropdownItem } from './type'

// KunDropdown is a click-triggered action menu (the WAI-ARIA menu-button
// pattern). It is deliberately NOT built on KunPopover: a menu needs
// role="menu"/menuitem semantics, roving tabindex, arrow-key navigation
// and focus management that Popover (role="dialog", internal-only open
// state) can't surface. So it wraps @floating-ui/vue directly for
// positioning — the same recipe Popover uses — while owning its own
// interaction + a11y layer. It reuses the ContextMenu item model
// (KunDropdownItem) and the shared variant-class table for visuals.
//
// There is intentionally no hover trigger: hover menus aren't a WAI-ARIA
// pattern (touch has no hover, keyboard/SR users can't reach them) and the
// use case here is "a list of clickable actions".
const props = withDefaults(
  defineProps<{
    items?: KunDropdownItem[]
    position?: Placement
    triggerClass?: string
    menuClass?: string
    minWidth?: number
    disabled?: boolean
  }>(),
  {
    items: () => [],
    position: 'bottom-start',
    triggerClass: '',
    menuClass: '',
    minWidth: 192,
    disabled: false,
  }
)

const emit = defineEmits<{
  (e: 'select', item: KunDropdownItem): void
  (e: 'open'): void
  (e: 'close'): void
}>()

const isOpen = ref(false)
// activeIndex tracks the roving-tabindex / focused menuitem (index into
// props.items). -1 = no item focused (menu container holds focus).
const activeIndex = ref(-1)
// Trigger is a div[role=button], mirroring KunPopover: consumers pass a
// <KunButton> into #trigger, so a wrapping <button> would nest buttons.
const triggerRef = ref<HTMLElement | null>(null)
const menuRef = ref<HTMLElement | null>(null)
const menuId = `kun-dropdown-${useId()}` // stable across SSR/client

const { floatingStyles } = useFloating(triggerRef, menuRef, {
  placement: computed(() => props.position),
  open: isOpen,
  whileElementsMounted: autoUpdate,
  // Position via top/left so the Transition's scale-* enter/leave classes
  // don't fight a translate3d transform on the same element (same reason
  // KunPopover sets transform: false).
  transform: false,
  middleware: [offset(6), flip(), shift({ padding: 8 })],
})

// Indices of non-disabled items, in order — the navigable set.
const enabledIndices = () =>
  props.items.reduce<number[]>((acc, item, i) => {
    if (!item.disabled) acc.push(i)
    return acc
  }, [])

// Live menuitem button elements (teleported, so query at call time).
const itemButtons = () =>
  Array.from(
    menuRef.value?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? []
  )

const focusItem = (index: number) => {
  activeIndex.value = index
  itemButtons()[index]?.focus()
}

const open = (focus: 'first' | 'last' | 'none' = 'none') => {
  if (props.disabled || props.items.length === 0) return
  if (!isOpen.value) {
    isOpen.value = true
    emit('open')
  }
  // Menu mounts on the next tick (v-if); resolve focus afterwards.
  nextTick(() => {
    const enabled = enabledIndices()
    if (focus === 'first' && enabled.length) {
      focusItem(enabled[0]!)
    } else if (focus === 'last' && enabled.length) {
      focusItem(enabled[enabled.length - 1]!)
    } else {
      activeIndex.value = -1
      menuRef.value?.focus()
    }
  })
}

const close = (returnFocus = false) => {
  if (!isOpen.value) return
  isOpen.value = false
  activeIndex.value = -1
  emit('close')
  if (returnFocus) nextTick(() => triggerRef.value?.focus())
}

const toggle = () => (isOpen.value ? close() : open('none'))

// Move roving focus by delta within the enabled set, wrapping around.
const move = (delta: number) => {
  const enabled = enabledIndices()
  if (!enabled.length) return
  const pos = enabled.indexOf(activeIndex.value)
  const nextPos = (pos + delta + enabled.length) % enabled.length
  // pos === -1 (nothing active yet) + delta 1 → first; + delta -1 → last.
  focusItem(enabled[nextPos === -1 ? enabled.length - 1 : nextPos]!)
}

const selectItem = (item: KunDropdownItem) => {
  if (item.disabled) return
  emit('select', item)
  close(true)
}

const onTriggerKeydown = (e: KeyboardEvent) => {
  if (props.disabled) return
  switch (e.key) {
    case 'ArrowDown':
    case 'Enter':
    case ' ':
      e.preventDefault() // also suppresses the synthetic click on Enter/Space
      open('first')
      break
    case 'ArrowUp':
      e.preventDefault()
      open('last')
      break
  }
}

const onMenuKeydown = (e: KeyboardEvent) => {
  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      move(1)
      break
    case 'ArrowUp':
      e.preventDefault()
      move(-1)
      break
    case 'Home': {
      e.preventDefault()
      const first = enabledIndices()[0]
      if (first !== undefined) focusItem(first)
      break
    }
    case 'End': {
      e.preventDefault()
      const enabled = enabledIndices()
      if (enabled.length) focusItem(enabled[enabled.length - 1]!)
      break
    }
    case 'Enter':
    case ' ':
      e.preventDefault()
      if (activeIndex.value >= 0) selectItem(props.items[activeIndex.value]!)
      break
    case 'Escape':
      e.preventDefault()
      close(true)
      break
    case 'Tab':
      // Teleported menu has no meaningful next tab stop; close + return to
      // trigger so tab order stays sane.
      e.preventDefault()
      close(true)
      break
  }
}

// Close on outside click — mirror KunPopover: anchor the listener on the
// trigger, bail when the click landed inside the teleported menu.
onClickOutside(triggerRef, (e) => {
  if (menuRef.value?.contains(e.target as Node)) return
  close()
})

// Safety net for Escape when focus somehow left the menu while open.
useEventListener('keydown', (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isOpen.value) close(true)
})

// focus: highlight matching each color's hover tint (light variant only
// defines hover:, which keyboard focus never triggers).
const focusTint: Record<KunUIColor, string> = {
  default: 'focus:bg-default/20',
  primary: 'focus:bg-primary/20',
  secondary: 'focus:bg-secondary/20',
  success: 'focus:bg-success/20',
  warning: 'focus:bg-warning/20',
  danger: 'focus:bg-danger/20',
  info: 'focus:bg-info/20',
}

const itemClass = (item: KunDropdownItem) =>
  cn(
    'relative flex w-full cursor-pointer items-center justify-start gap-2 overflow-hidden rounded-lg px-3 py-1.5 text-sm font-medium outline-none transition-colors',
    kunVariantClasses('light', item.color || 'default'),
    focusTint[item.color || 'default'],
    item.disabled && 'pointer-events-none cursor-not-allowed opacity-50'
  )

defineExpose({
  open: () => open('none'),
  close: () => close(),
  toggle,
})
</script>

<template>
  <div class="relative inline-flex">
    <div
      ref="triggerRef"
      role="button"
      :tabindex="disabled ? -1 : 0"
      :class="
        cn(
          'inline-flex cursor-pointer items-center',
          disabled && 'cursor-not-allowed opacity-50',
          triggerClass
        )
      "
      aria-haspopup="menu"
      :aria-expanded="isOpen"
      :aria-disabled="disabled || undefined"
      :aria-controls="isOpen ? menuId : undefined"
      @click="disabled || toggle()"
      @keydown="onTriggerKeydown"
    >
      <slot name="trigger" />
    </div>

    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-150 ease-out"
        enter-from-class="opacity-0 scale-95"
        enter-to-class="opacity-100 scale-100"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100 scale-100"
        leave-to-class="opacity-0 scale-95"
      >
        <div
          v-if="isOpen && items.length"
          ref="menuRef"
          :id="menuId"
          role="menu"
          aria-orientation="vertical"
          tabindex="-1"
          :class="
            cn(
              'border-default-200 bg-background/95 z-kun-popover rounded-xl border p-1 text-sm shadow-2xl outline-none backdrop-blur',
              menuClass
            )
          "
          :style="[floatingStyles, { minWidth: `${minWidth}px` }]"
          @keydown="onMenuKeydown"
        >
          <button
            v-for="(item, i) in items"
            :key="item.key"
            type="button"
            role="menuitem"
            :tabindex="i === activeIndex ? 0 : -1"
            :disabled="item.disabled"
            :aria-disabled="item.disabled || undefined"
            :class="itemClass(item)"
            @click="selectItem(item)"
            @mouseenter="!item.disabled && focusItem(i)"
          >
            <KunIcon v-if="item.icon" :name="item.icon" class="text-base" />
            <span>{{ item.label }}</span>
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
