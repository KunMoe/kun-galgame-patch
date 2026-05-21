<script setup lang="ts">
import { useRipple } from '../ripple/useRipple'
import type { KunUIColor } from '../ui/type'

// KunCard renders as one of three elements depending on which props
// are present (priority: href > clickable > div):
//   1. `href`      → <NuxtLink>
//   2. `clickable` → <button> (emits @click)
//   3. neither     → static <div>
interface Props {
  isHoverable?: boolean
  clickable?: boolean
  href?: string
  isTransparent?: boolean
  bordered?: boolean
  className?: string
  contentClass?: string
  rounded?: 'none' | 'sm' | 'md' | 'lg' | 'full'
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
  rounded: 'lg',
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
  if (!isInteractive.value) return
  onClick(event)
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

const roundedClasses = computed(() => {
  switch (props.rounded) {
    case 'none':
      return 'rounded-none'
    case 'sm':
      return 'rounded-sm'
    case 'md':
      return 'rounded-md'
    case 'lg':
      return 'rounded-lg'
    case 'full':
      return 'rounded-full'
    default:
      return 'rounded-lg'
  }
})
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
        roundedClasses,
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
