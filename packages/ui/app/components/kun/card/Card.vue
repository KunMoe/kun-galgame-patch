<script setup lang="ts">
import { useRipple } from '../ripple/useRipple'
import { kunRoundedClasses, useResolvedRounded } from '../ui/rounded'
import type { KunUIColor, KunUIRounded } from '../ui/type'

// KunCard renders as one of three elements depending on which props
// are present (priority: href > clickable > div):
//   1. `href`      → <NuxtLink>
//   2. `clickable` → <button> (visual cursor/active-scale + ripple)
//   3. neither     → static <div>
//
// `@click` is emitted in ALL three modes — see handleKunCardClick.
// Earlier versions short-circuited the emit on non-interactive cards,
// which broke the imperative pattern `<KunCard @click="navigateTo(...)">`
// used to avoid `<a>`-inside-`<a>` hydration mismatches (e.g. comment
// cards rendering markdown that may contain @-mention links). Only the
// ripple effect + cursor/scale styling remain gated on isInteractive —
// those are visual cues that imply "this is a primary nav target",
// which a pure event listener shouldn't claim.
interface Props {
  isHoverable?: boolean
  clickable?: boolean
  href?: string
  isTransparent?: boolean
  bordered?: boolean
  className?: string
  contentClass?: string
  rounded?: KunUIRounded
  color?: KunUIColor | 'background'
  darkBorder?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  clickable: false,
  href: undefined,
  isHoverable: false,
  isTransparent: false,
  bordered: true,
  className: '',
  contentClass: '',
  rounded: undefined,
  color: 'background',
  darkBorder: false
})

const emit = defineEmits<{
  click: [event: MouseEvent]
}>()

const { ripples, onClick } = useRipple()

const isInteractive = computed(() => !!props.href || props.clickable)
const renderAs = computed(() => {
  if (props.href) return defineNuxtLink({})
  if (props.clickable) return 'button'
  return 'div'
})

const handleKunCardClick = (event: MouseEvent) => {
  // Ripple is a visual affordance for "primary nav target" cards — only
  // run it when the card is rendered as <NuxtLink>/<button>, i.e. when
  // it visually advertises clickability via cursor-pointer + active-scale.
  if (isInteractive.value) onClick(event)
  // Emit unconditionally so plain `<KunCard @click="...">` works. The
  // declared `click` emit means Vue stops auto-forwarding the native
  // click from the root element, so we have to fire it ourselves —
  // a missing emit here silently swallows the listener (the bug this
  // file's top comment describes).
  emit('click', event)
}

const colorClasses: Record<KunUIColor | 'background', string> = {
  background: 'bg-background',
  default: 'bg-default-100/30',
  primary: 'bg-primary-100/30 border-primary-300',
  secondary: 'bg-secondary-100/30 border-secondary-300',
  success: 'bg-success-100/30 border-success-300',
  warning: 'bg-warning-100/30 border-warning-300',
  danger: 'bg-danger-100/30 border-danger-300',
  info: 'bg-info-100/30 border-info-300'
}

const rounded = useResolvedRounded(() => props.rounded)
const roundedClass = computed(() => kunRoundedClasses[rounded.value])
</script>

<template>
  <component
    :is="renderAs"
    :class="
      cn(
        'relative flex flex-col gap-3 p-3 backdrop-blur-[var(--kun-background-blur)] transition-all duration-200',
        isHoverable && 'hover:bg-default-100',
        bordered && 'border-default/20 border',
        darkBorder &&
          cn(
            'dark:border-default-200 border border-transparent',
            bordered && 'border-default/20'
          ),
        isInteractive && 'cursor-pointer overflow-hidden active:scale-[0.97] text-left',
        isTransparent ? 'backdrop-blur-none' : colorClasses[props.color],
        roundedClass,
        className
      )
    "
    :to="props.href"
    :type="props.clickable && !props.href ? 'button' : undefined"
    @click="handleKunCardClick"
  >
    <div v-if="$slots.header" class="border-b">
      <slot name="header" />
    </div>

    <div v-if="$slots.cover" class="w-full">
      <slot name="cover" />
    </div>

    <div
      :class="cn('flex h-full flex-col justify-between gap-1', contentClass)"
    >
      <slot />
    </div>

    <div v-if="$slots.footer" class="bg-default-100 border-t px-3 py-2">
      <slot name="footer" />
    </div>

    <KunRipple :ripples="ripples" />
  </component>
</template>
