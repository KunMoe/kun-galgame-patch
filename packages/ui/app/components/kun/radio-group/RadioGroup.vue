<script
  setup
  lang="ts"
  generic="T extends KunRadioValue = KunRadioValue"
>
import { computed, nextTick, ref } from 'vue'
import {
  kunBgClasses,
  kunBorderClasses,
  kunRingClasses,
  kunSoftBgClasses,
} from '../ui/variants'
import { kunRoundedClasses, useResolvedRounded } from '../ui/rounded'
import type {
  KunRadioGroupProps,
  KunRadioOption,
  KunRadioValue,
} from './type'

const props = withDefaults(defineProps<KunRadioGroupProps<T>>(), {
  variant: 'classic',
  orientation: 'vertical',
  color: 'primary',
  size: 'md',
  rounded: undefined,
  disabled: false,
  error: '',
  ariaLabel: '',
  label: '',
  className: ''
})

const modelValue = defineModel<T>({ required: true })

const emits = defineEmits<{
  change: [value: T, index: number]
}>()

const rounded = useResolvedRounded(() => props.rounded, 'md')
const cardRoundedClass = computed(() => kunRoundedClasses[rounded.value])

const kunUniqueId = useKunUniqueId('kun-radio-group')
const labelId = computed(() => `${kunUniqueId}-label`)
const itemRefs = ref<Array<HTMLElement | null>>([])

const isOptionDisabled = (option: KunRadioOption<T>) =>
  props.disabled || option.disabled === true

// Roving tabindex: only one item in the group is reachable via Tab.
// Prefers the selected option, falls back to the first non-disabled
// option so the group is always reachable even before any selection
// is made.
const focusableIndex = computed(() => {
  const selectedIdx = props.options.findIndex(
    (o) => o.value === modelValue.value
  )
  if (selectedIdx >= 0 && !isOptionDisabled(props.options[selectedIdx]!)) {
    return selectedIdx
  }
  return props.options.findIndex((o) => !isOptionDisabled(o))
})

const sizeClasses = computed(() => {
  switch (props.size) {
    case 'xs':
      return { indicator: 'size-3', dot: 'size-1.5', text: 'text-xs', gap: 'gap-1.5' }
    case 'sm':
      return { indicator: 'size-3.5', dot: 'size-1.5', text: 'text-sm', gap: 'gap-2' }
    case 'md':
      return { indicator: 'size-4', dot: 'size-2', text: 'text-sm', gap: 'gap-2' }
    case 'lg':
      return { indicator: 'size-5', dot: 'size-2.5', text: 'text-base', gap: 'gap-2.5' }
    case 'xl':
      return { indicator: 'size-6', dot: 'size-3', text: 'text-lg', gap: 'gap-3' }
    default:
      return { indicator: 'size-4', dot: 'size-2', text: 'text-sm', gap: 'gap-2' }
  }
})

const selectOption = (option: KunRadioOption<T>, index: number) => {
  if (isOptionDisabled(option)) {
    return
  }
  if (modelValue.value === option.value) {
    return
  }
  modelValue.value = option.value
  emits('change', option.value, index)
}

// WAI-ARIA radio pattern: arrow keys move focus AND activate the
// target radio (unlike tab/menu navigation which just moves focus).
// Disabled items are skipped so the caret never stalls on them.
const focusSibling = (from: number, delta: number) => {
  const total = props.options.length
  if (total === 0) {
    return
  }
  let cursor = from
  for (let step = 0; step < total; step++) {
    cursor = (cursor + delta + total) % total
    const candidate = props.options[cursor]
    if (candidate && !isOptionDisabled(candidate)) {
      selectOption(candidate, cursor)
      nextTick(() => itemRefs.value[cursor]?.focus())
      return
    }
  }
}

const onKeydown = (event: KeyboardEvent, index: number) => {
  switch (event.key) {
    case 'ArrowDown':
    case 'ArrowRight':
      event.preventDefault()
      focusSibling(index, 1)
      break
    case 'ArrowUp':
    case 'ArrowLeft':
      event.preventDefault()
      focusSibling(index, -1)
      break
    case ' ':
    case 'Enter': {
      event.preventDefault()
      const option = props.options[index]
      if (option) {
        selectOption(option, index)
      }
      break
    }
  }
}
</script>

<template>
  <div :class="cn('w-full', className)">
    <div
      v-if="label"
      :id="labelId"
      class="text-default-700 mb-2 block text-sm font-medium"
    >
      {{ label }}
    </div>

    <div
      role="radiogroup"
      :aria-label="label ? undefined : ariaLabel || 'radio group'"
      :aria-labelledby="label ? labelId : undefined"
      :aria-disabled="disabled || undefined"
      :class="
        cn(
          'flex',
          orientation === 'vertical' ? 'flex-col gap-2' : 'flex-row flex-wrap gap-3'
        )
      "
    >
      <!-- ============ classic variant: dot + label ============ -->
      <template v-if="variant === 'classic'">
        <label
          v-for="(option, index) in options"
          :key="String(option.value)"
          :ref="(el) => (itemRefs[index] = el as HTMLElement | null)"
          role="radio"
          :aria-checked="modelValue === option.value"
          :aria-disabled="isOptionDisabled(option) || undefined"
          :tabindex="focusableIndex === index && !isOptionDisabled(option) ? 0 : -1"
          :class="
            cn(
              'group inline-flex cursor-pointer items-center rounded-md p-1 transition-colors focus:outline-none',
              kunRingClasses[color],
              'focus:ring-2',
              sizeClasses.gap,
              sizeClasses.text,
              isOptionDisabled(option) && 'cursor-not-allowed opacity-50'
            )
          "
          @click="selectOption(option, index)"
          @keydown="(e) => onKeydown(e, index)"
        >
          <span
            :class="
              cn(
                'inline-flex shrink-0 items-center justify-center rounded-full border-2 transition-colors',
                sizeClasses.indicator,
                modelValue === option.value
                  ? kunBorderClasses[color]
                  : 'border-default-300 group-hover:border-default-400'
              )
            "
          >
            <span
              v-if="modelValue === option.value"
              :class="
                cn(
                  'block rounded-full',
                  sizeClasses.dot,
                  kunBgClasses[color]
                )
              "
            />
          </span>
          <span class="flex flex-col">
            <span class="text-foreground">{{ option.label }}</span>
            <span
              v-if="option.description"
              class="text-default-500 text-xs"
            >
              {{ option.description }}
            </span>
          </span>
        </label>
      </template>

      <!-- ============ card variant: bordered card with tint ============ -->
      <template v-else>
        <div
          v-for="(option, index) in options"
          :key="String(option.value)"
          :ref="(el) => (itemRefs[index] = el as HTMLElement | null)"
          role="radio"
          :aria-checked="modelValue === option.value"
          :aria-disabled="isOptionDisabled(option) || undefined"
          :tabindex="focusableIndex === index && !isOptionDisabled(option) ? 0 : -1"
          :class="
            cn(
              'relative flex cursor-pointer items-start border-2 p-3 transition-all focus:outline-none focus:ring-2',
              kunRingClasses[color],
              sizeClasses.gap,
              sizeClasses.text,
              cardRoundedClass,
              orientation === 'horizontal' && 'flex-1 min-w-[8rem]',
              modelValue === option.value
                ? cn(kunBorderClasses[color], kunSoftBgClasses[color])
                : 'border-default-200 hover:border-default-300 bg-content1',
              isOptionDisabled(option) && 'cursor-not-allowed opacity-50'
            )
          "
          @click="selectOption(option, index)"
          @keydown="(e) => onKeydown(e, index)"
        >
          <span
            :class="
              cn(
                'mt-0.5 inline-flex shrink-0 items-center justify-center rounded-full border-2 transition-colors',
                sizeClasses.indicator,
                modelValue === option.value
                  ? kunBorderClasses[color]
                  : 'border-default-300'
              )
            "
          >
            <span
              v-if="modelValue === option.value"
              :class="
                cn(
                  'block rounded-full',
                  sizeClasses.dot,
                  kunBgClasses[color]
                )
              "
            />
          </span>
          <div class="flex flex-col">
            <span class="text-foreground font-medium">{{ option.label }}</span>
            <span
              v-if="option.description"
              class="text-default-500 mt-0.5 text-xs"
            >
              {{ option.description }}
            </span>
          </div>
        </div>
      </template>
    </div>

    <p v-if="error" class="text-danger mt-2 text-sm">{{ error }}</p>
  </div>
</template>
