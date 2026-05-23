<script setup lang="ts">
import { useEventListener, useMediaQuery } from '@vueuse/core'
import { useFocusTrap } from '@vueuse/integrations/useFocusTrap'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useBodyScrollLock } from '../../../composables/useBodyScrollLock'
import { kunRoundedClasses, useResolvedRounded } from '../ui/rounded'
import type {
  KunDrawerPlacement,
  KunDrawerProps,
  KunDrawerSize,
} from './type'

const props = withDefaults(defineProps<KunDrawerProps>(), {
  placement: 'right',
  responsive: true,
  size: 'md',
  title: '',
  isDismissable: true,
  isShowCloseButton: true,
  withContainer: true,
  rounded: undefined,
  className: '',
  innerClassName: '',
})

// Match Tailwind's `md` breakpoint (default 48rem = 768px). Below this
// width the drawer becomes a bottom sheet by default — that's the
// "right on desktop, bottom on mobile" pattern almost every consumer
// wants. Reactive: rotating from portrait to landscape across the
// 768px boundary re-evaluates effectivePlacement automatically.
//
// Cost: useMediaQuery sets up exactly one matchMedia listener per
// component instance. SSR-safe (the v-model gate means the drawer
// isn't even in the DOM during SSR for most cases; on first client
// paint, isMobile resolves on the next tick).
const isMobile = useMediaQuery('(max-width: 47.99rem)')

const effectivePlacement = computed<KunDrawerPlacement>(() =>
  props.responsive && isMobile.value ? 'bottom' : props.placement
)

// 'lg' built-in matches Modal — drawer is a large surface where 'lg'
// looks better than the global 'md' fallback.
const rounded = useResolvedRounded(() => props.rounded, 'lg')
const roundedClass = computed(() => kunRoundedClasses[rounded.value])

const modelValue = defineModel<boolean>({ required: true })

const emits = defineEmits<{
  close: []
}>()

const kunUniqueId = useKunUniqueId('kun-drawer')
const titleId = computed(() => `${kunUniqueId}-title`)

// ──────────────────────────────────────────
// Geometry — static lookup maps keep Tailwind JIT happy
// ──────────────────────────────────────────

// Width tokens (active when placement is left/right).
const widthClassMap: Record<KunDrawerSize, string> = {
  sm: 'w-80',           // 320px
  md: 'w-96',           // 384px
  lg: 'w-[32rem]',      // 512px
  xl: 'w-[40rem]',      // 640px
  full: 'w-full',
}

// Height tokens (active when placement is top/bottom).
const heightClassMap: Record<KunDrawerSize, string> = {
  sm: 'h-80',
  md: 'h-96',
  lg: 'h-[32rem]',
  xl: 'h-[40rem]',
  full: 'h-full',
}

// Panel anchoring per placement. Only the corners FACING THE INSIDE
// are rounded — the edge anchored to the viewport stays square so the
// drawer looks flush with its edge instead of having a visible gap.
const placementAnchorMap: Record<KunDrawerPlacement, string> = {
  left: 'left-0 top-0 h-full',
  right: 'right-0 top-0 h-full',
  top: 'top-0 left-0 w-full',
  bottom: 'bottom-0 left-0 w-full',
}

// Round only the corners on the side opposite the anchored edge.
// `rounded-r-*` etc. via the kun radius vars — written explicitly
// (not algorithmically) to stay JIT-safe.
const placementRoundedSideMap: Record<KunDrawerPlacement, string> = {
  left: 'rounded-r-kun-lg',
  right: 'rounded-l-kun-lg',
  top: 'rounded-b-kun-lg',
  bottom: 'rounded-t-kun-lg',
}

const panelGeometry = computed(() => {
  const placement = effectivePlacement.value
  const isHorizontal = placement === 'left' || placement === 'right'
  const sizeClass = isHorizontal
    ? widthClassMap[props.size]
    : heightClassMap[props.size]
  return [
    'absolute',
    placementAnchorMap[placement],
    sizeClass,
    // For the `full` size we don't round any corner — the panel
    // covers the entire opposite edge so curved corners look weird.
    props.size === 'full' ? '' : placementRoundedSideMap[placement],
  ]
    .filter(Boolean)
    .join(' ')
})

// Distinguishes which CSS transition variant to apply (kun-drawer-left
// / -right / -top / -bottom). Each variant slides in from its edge.
// Reads effectivePlacement so the slide direction follows the
// responsive switch — on mobile the drawer slides UP from the bottom
// regardless of the `placement` prop's authored value.
const transitionName = computed(() => `kun-drawer-${effectivePlacement.value}`)

// ──────────────────────────────────────────
// Modal mechanics — body scroll lock + focus trap + Escape handling
// ──────────────────────────────────────────

const { lock, unlock } = useBodyScrollLock()
let locked = false
const applyLock = (shouldLock: boolean) => {
  if (shouldLock && !locked) {
    lock()
    locked = true
  } else if (!shouldLock && locked) {
    unlock()
    locked = false
  }
}

// Focus trap on the entire overlay — Tab/Shift+Tab stay inside the
// drawer subtree. escapeDeactivates: false because Drawer owns the
// Escape handler (routes through isDismissable). allowOutsideClick
// permits the backdrop click handler to fire.
const trapEl = ref<HTMLElement | null>(null)
const { activate, deactivate } = useFocusTrap(trapEl, {
  immediate: false,
  escapeDeactivates: false,
  allowOutsideClick: true,
  returnFocusOnDeactivate: true,
})

const handleClose = () => {
  if (props.isDismissable) {
    modelValue.value = false
    emits('close')
  }
}

const handleCloseButton = () => {
  // Close button always closes regardless of isDismissable — that
  // matches Modal's behavior. isDismissable controls backdrop +
  // Escape only.
  modelValue.value = false
  emits('close')
}

useEventListener('keydown', (e: KeyboardEvent) => {
  if (e.key === 'Escape' && modelValue.value) {
    handleClose()
  }
})

watch(modelValue, async (v) => {
  applyLock(v)
  if (v) {
    await nextTick()
    activate()
  } else {
    deactivate()
  }
})

onMounted(async () => {
  if (modelValue.value) {
    applyLock(true)
    await nextTick()
    activate()
  }
})

onUnmounted(() => {
  applyLock(false)
  deactivate()
})
</script>

<template>
  <Teleport to="body">
    <Transition :name="transitionName">
      <div
        v-if="modelValue"
        ref="trapEl"
        :class="
          cn(
            'z-kun-modal fixed inset-0',
            className
          )
        "
        role="dialog"
        aria-modal="true"
        :aria-labelledby="title ? titleId : undefined"
        tabindex="0"
      >
        <!-- Backdrop — separate from panel so panel slide animation
             doesn't fight backdrop opacity fade. -->
        <div
          class="bg-default-800/70 dark:bg-background/70 absolute inset-0 transition-opacity"
          @click="handleClose"
        />

        <!-- Drawer panel -->
        <div
          :class="
            cn(
              'bg-content1 flex flex-col border shadow-2xl',
              panelGeometry,
              innerClassName,
              size === 'full' ? roundedClass : ''
            )
          "
          @click.stop
        >
          <!-- Header — appears if title prop OR header slot OR close
               button needs anchor. -->
          <header
            v-if="title || $slots.header || isShowCloseButton"
            class="border-default-200 flex items-center justify-between border-b px-6 py-4"
          >
            <div class="flex min-w-0 flex-1 items-center gap-2">
              <h2
                v-if="title"
                :id="titleId"
                class="text-foreground truncate text-lg font-semibold"
              >
                {{ title }}
              </h2>
              <slot name="header" />
            </div>

            <KunButton
              v-if="isShowCloseButton"
              color="default"
              variant="light"
              size="sm"
              rounded="full"
              :is-icon-only="true"
              aria-label="关闭抽屉"
              class-name="shrink-0"
              @click="handleCloseButton"
            >
              <KunIcon name="lucide:x" />
            </KunButton>
          </header>

          <!-- Body -->
          <div
            v-if="withContainer"
            class="scrollbar-hide flex-1 overflow-y-auto p-6"
          >
            <slot />
          </div>
          <slot v-else />

          <!-- Footer (optional) -->
          <footer
            v-if="$slots.footer"
            class="border-default-200 border-t px-6 py-4"
          >
            <slot name="footer" />
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* All four placement transitions share the same easing + duration so
 * the user gets a consistent feel regardless of which edge the drawer
 * comes from. The backdrop fade is driven by `transition-opacity` on
 * the backdrop element above; this block only controls the panel
 * slide. */
.kun-drawer-left-enter-active,
.kun-drawer-left-leave-active,
.kun-drawer-right-enter-active,
.kun-drawer-right-leave-active,
.kun-drawer-top-enter-active,
.kun-drawer-top-leave-active,
.kun-drawer-bottom-enter-active,
.kun-drawer-bottom-leave-active {
  transition: opacity 0.3s ease;
}

.kun-drawer-left-enter-active > div:last-child,
.kun-drawer-left-leave-active > div:last-child,
.kun-drawer-right-enter-active > div:last-child,
.kun-drawer-right-leave-active > div:last-child,
.kun-drawer-top-enter-active > div:last-child,
.kun-drawer-top-leave-active > div:last-child,
.kun-drawer-bottom-enter-active > div:last-child,
.kun-drawer-bottom-leave-active > div:last-child {
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.kun-drawer-left-enter-from,
.kun-drawer-left-leave-to {
  opacity: 0;
}
.kun-drawer-left-enter-from > div:last-child,
.kun-drawer-left-leave-to > div:last-child {
  transform: translateX(-100%);
}

.kun-drawer-right-enter-from,
.kun-drawer-right-leave-to {
  opacity: 0;
}
.kun-drawer-right-enter-from > div:last-child,
.kun-drawer-right-leave-to > div:last-child {
  transform: translateX(100%);
}

.kun-drawer-top-enter-from,
.kun-drawer-top-leave-to {
  opacity: 0;
}
.kun-drawer-top-enter-from > div:last-child,
.kun-drawer-top-leave-to > div:last-child {
  transform: translateY(-100%);
}

.kun-drawer-bottom-enter-from,
.kun-drawer-bottom-leave-to {
  opacity: 0;
}
.kun-drawer-bottom-enter-from > div:last-child,
.kun-drawer-bottom-leave-to > div:last-child {
  transform: translateY(100%);
}
</style>
