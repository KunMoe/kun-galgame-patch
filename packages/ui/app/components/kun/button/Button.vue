<script setup lang="ts">
import { useRipple } from '../ripple/useRipple'
import { kunVariantClasses } from '../ui/variants'
import { kunRoundedClasses, useResolvedRounded } from '../ui/rounded'
import type { KunButtonProps } from './type'

const props = withDefaults(defineProps<KunButtonProps>(), {
  variant: 'solid',
  color: 'primary',
  size: 'md',
  rounded: 'lg',
  type: 'button',
  disabled: false,
  loading: false,
  fullWidth: false,
  isIconOnly: false,
  icon: false,
  iconPosition: 'left',
  className: '',
  href: '',
  target: '_self',
  ariaLabel: ''
})

const emits = defineEmits<{
  click: [event: MouseEvent]
}>()

const slots = useSlots()

const computedAriaLabel = computed(() => {
  if (props.ariaLabel) {
    return props.ariaLabel
  }

  if (props.isIconOnly) {
    // if (import.meta.dev) {
    //   console.warn(
    //     `[KunButton] An icon-only button should have an explicit 'ariaLabel' prop for accessibility.`
    //   )
    // }
    return 'button'
  }

  if (slots.default) {
    const slotText = extractTextFromVNodes(slots.default()).trim()
    return slotText || ''
  }

  return ''
})

const sizeClasses = computed(() => {
  switch (props.size) {
    case 'xs':
      return 'text-xs px-2 py-1'
    case 'sm':
      return 'text-sm px-3 py-1.5'
    case 'md':
      return 'text-sm px-4 py-2'
    case 'lg':
      return 'text-base px-5 py-2.5'
    case 'xl':
      return 'text-lg px-6 py-3'
    default:
      return 'text-sm px-4 py-2'
  }
})

// Button delegates the entire variant × color matrix to the shared
// kunVariantClasses table in ui/variants.ts. Any new color or variant
// added there propagates here without local edits. (v0.1.0 added the
// `info` color but Button kept its own 6-color local table, which TS
// rejected as missing the `info` property — fixed by deleting the
// local table entirely.)
const colorClasses = computed(() =>
  kunVariantClasses(props.variant, props.color)
)

const rounded = useResolvedRounded(() => props.rounded)
const roundedClass = computed(() => kunRoundedClasses[rounded.value])

const isIconOnlyClasses = computed(() => {
  if (!props.isIconOnly) {
    return ''
  }
  switch (props.size) {
    case 'xs':
      return 'p-1'
    case 'sm':
      return 'p-1.5'
    case 'md':
      return 'p-2'
    case 'lg':
      return 'p-2.5'
    case 'xl':
      return 'p-3'
    default:
      return 'p-2'
  }
})

const { ripples, onClick } = useRipple()

const handleKunButtonClick = (event: MouseEvent) => {
  onClick(event)
  emits('click', event)
}
</script>

<template>
  <component
    :is="props.href ? defineNuxtLink({}) : 'button'"
    :class="
      cn(
        'relative inline-flex cursor-pointer items-center justify-center gap-1 overflow-hidden font-medium transition-all hover:opacity-80 active:scale-[0.97] disabled:opacity-50',
        sizeClasses,
        colorClasses,
        roundedClass,
        fullWidth ? 'w-full' : '',
        isIconOnlyClasses,
        disabled && 'cursor-not-allowed hover:bg-none',
        className
      )
    "
    :to="props.href"
    :target="props.target"
    :disabled="disabled || loading"
    :role="props.href ? 'link' : 'button'"
    :type="type"
    :aria-label="computedAriaLabel"
    @click="handleKunButtonClick"
  >
    <KunIcon
      class="text-sm"
      v-if="loading"
      name="svg-spinners:90-ring-with-bg"
    />
    <span v-if="icon && iconPosition === 'left'" class="mr-2">
      <slot name="icon" />
    </span>
    <slot />
    <span v-if="icon && iconPosition === 'right'" class="ml-2">
      <slot name="icon" />
    </span>

    <KunRipple :ripples="ripples" />
  </component>
</template>
