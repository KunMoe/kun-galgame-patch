<script setup lang="ts">
// Hover-triggered dropdown menu for the top bar.
//
// Why a dedicated component (not KunTooltip / KunPopover):
//   - KunTooltip is for non-interactive informational text — its body has
//     no "stay open while pointer is over me" handshake, and adding one
//     overloads the tooltip semantic.
//   - KunPopover is click-triggered — it doesn't auto-open on hover, which
//     is the UX product wants for top-bar nav (think "Products ▾" menus
//     on most marketing sites).
//
// Mechanics:
//   - mouseenter on trigger OR body  → open (cancel any pending close)
//   - mouseleave on trigger OR body  → schedule close after `closeDelay`ms
//   - Because trigger and body live in different DOM subtrees (body is
//     Teleported to <body> so it can escape the nav's stacking context),
//     leaving one then entering the other fires (leave, enter) almost
//     simultaneously — the close timer is cancelled before it fires.
//   - The non-zero delay (default 200ms) gives the pointer time to cross
//     the 8px gap (offset middleware) between trigger and body. Slower
//     pointers that DO let the timer fire are caught by the body's
//     mouseenter forcing isOpen back to true (Vue Transition cancels the
//     leave animation).

import {
  useFloating,
  autoUpdate,
  offset as offsetMw,
  flip,
  shift,
  type Placement
} from '@floating-ui/vue'

interface Props {
  position?: Placement
  closeDelay?: number
  // Apply `text-primary` to the trigger when this is true. Lets the
  // caller forward the active-route highlight (same look as the sibling
  // <NuxtLink> nav items) without us having to know about the route.
  active?: boolean
  className?: string
}

const props = withDefaults(defineProps<Props>(), {
  position: 'bottom',
  closeDelay: 200,
  active: false,
  className: ''
})

const triggerRef = ref<HTMLElement | null>(null)
const menuRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)

let closeTimer: ReturnType<typeof setTimeout> | null = null

const cancelClose = () => {
  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = null
  }
}

const open = () => {
  cancelClose()
  isOpen.value = true
}

const scheduleClose = () => {
  cancelClose()
  closeTimer = setTimeout(() => {
    isOpen.value = false
  }, props.closeDelay)
}

const { floatingStyles } = useFloating(triggerRef, menuRef, {
  placement: props.position,
  open: isOpen,
  whileElementsMounted: autoUpdate,
  middleware: [offsetMw(8), flip(), shift({ padding: 8 })]
})
</script>

<template>
  <div
    ref="triggerRef"
    :class="
      cn(
        'inline-flex items-center',
        active ? 'text-primary' : 'text-foreground',
        className
      )
    "
    @mouseenter="open"
    @mouseleave="scheduleClose"
  >
    <slot />

    <!-- Teleport so the menu escapes the nav's `backdrop-blur` stacking
         context (same reason MobileMenu teleports — without this the
         floating menu's own backdrop / shadow stack against the nav's
         rendered surface and the visual feels wrong). -->
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
          v-if="isOpen"
          ref="menuRef"
          :class="
            cn(
              'bg-content1 border-default-200 z-kun-popover hidden rounded-xl border shadow-lg sm:block'
            )
          "
          :style="floatingStyles"
          role="menu"
          @mouseenter="open"
          @mouseleave="scheduleClose"
        >
          <slot name="content" />
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
