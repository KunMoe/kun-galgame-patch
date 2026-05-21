<script
  setup
  lang="ts"
  generic="T extends KunSelectValue = KunSelectValue"
>
import { computed, ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import {
  useFloating,
  autoUpdate,
  offset,
  flip,
  shift,
  size,
} from '@floating-ui/vue'
import type { KunSelectProps, KunSelectValue } from './type'
import { kunRoundedClasses, useResolvedRounded } from '../ui/rounded'

const props = withDefaults(defineProps<KunSelectProps<T>>(), {
  placeholder: '',
  label: '',
  disabled: false,
  error: '',
  darkBorder: true,
  ariaLabel: '',
  className: '',
  rounded: undefined
})

const rounded = useResolvedRounded(() => props.rounded)
const roundedClass = computed(() => kunRoundedClasses[rounded.value])

// `required: true` makes the model `Ref<T>` (without `undefined`), so
// consumers' @update:model-value callbacks can type the arg as `T`
// without TS rejecting it. Without `required` Vue 3.5 infers
// `Ref<T | undefined>` by spec.
const modelValue = defineModel<T>({ required: true })

const emit = defineEmits<{
  set: [value: T, index: number]
}>()

const kunUniqueId = useKunUniqueId('kun-select')
const isOpen = ref(false)
const buttonRef = ref<HTMLElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)

const { floatingStyles } = useFloating(buttonRef, dropdownRef, {
  placement: 'bottom-start',
  open: isOpen,
  whileElementsMounted: autoUpdate,
  // Position via top/left instead of transform so Vue Transition's
  // transform-based enter/leave classes (-translate-y-1, scale-95)
  // don't fight floating-ui's translate3d. Layout-cost trade-off is
  // negligible for click-open dropdowns. Width is set explicitly by
  // the size() middleware below so the docs' "fixed width or
  // max-content" requirement is already satisfied.
  // See floating-ui.com/docs/useFloating#transform.
  transform: false,
  middleware: [
    offset(4),
    flip(),
    shift({ padding: 8 }),
    // size middleware matches the dropdown width to the trigger and
    // caps height to the available viewport space so the list scrolls
    // instead of overflowing the screen.
    size({
      apply({ rects, elements, availableHeight }) {
        Object.assign(elements.floating.style, {
          width: `${rects.reference.width}px`,
          maxHeight: `${Math.min(240, availableHeight - 8)}px`,
        })
      },
    }),
  ],
})

onClickOutside(buttonRef, (event) => {
  if (dropdownRef.value?.contains(event.target as Node)) return
  isOpen.value = false
})

const selectedLabel = computed(() => {
  const selected = props.options.find(
    (option) => option.value === modelValue.value
  )
  return selected?.label
})

const toggle = () => {
  if (!props.disabled) isOpen.value = !isOpen.value
}

const selectOption = (value: T, index: number) => {
  modelValue.value = value
  emit('set', value, index)
  isOpen.value = false
}
</script>

<template>
  <div :class="cn('relative w-full', props.className)">
    <label
      v-if="label"
      :for="kunUniqueId"
      class="mb-2 block text-sm font-medium"
    >
      {{ label }}
    </label>

    <button
      ref="buttonRef"
      :id="kunUniqueId"
      type="button"
      :aria-label="props.ariaLabel || 'select'"
      :aria-expanded="isOpen"
      :aria-haspopup="'listbox'"
      :class="
        cn(
          'focus:border-primary focus:ring-primary flex w-full cursor-pointer items-center justify-between px-3 py-2 text-left text-sm focus:ring-1 focus:outline-none',
          roundedClass,
          darkBorder && 'dark:border-default-200 border-default/20 border',
          disabled && 'bg-default-100 cursor-not-allowed'
        )
      "
      @click="toggle"
      :disabled="disabled"
    >
      <span class="block min-w-0 flex-1 truncate">
        {{ selectedLabel || placeholder }}
      </span>
      <KunIcon
        name="lucide:chevron-down"
        class="pointer-events-none shrink-0"
        :class="
          cn('text-inherit transition-transform', isOpen ? 'rotate-180' : '')
        "
      />
    </button>

    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-150 ease-out"
        enter-from-class="opacity-0 -translate-y-1"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 -translate-y-1"
      >
        <div
          v-if="isOpen"
          ref="dropdownRef"
          :style="floatingStyles"
          :class="cn('bg-content1 border-default-200 z-kun-popover flex flex-col overflow-hidden border p-1 shadow-lg', roundedClass)"
        >
          <!-- flex-col + overflow-hidden on the outer floating div, plus
               flex-1 min-h-0 on the inner <ul> — together force the
               option list to honor the maxHeight that floating-ui's
               size() middleware writes inline. Without min-h-0 the
               flex item's implicit min-height: auto (= content height)
               wins and the <ul> stretches past the parent's maxHeight;
               the overflow-y-auto then never triggers because nothing
               is overflowing the <ul> itself. Same family of bug as
               the v0.4.7 "min-w-0 unsticks truncate in flex" fix —
               flex item min-* defaults need explicit override on both
               axes whenever a parent dimension constraint must propagate. -->
          <ul
            class="scrollbar-hide min-h-0 flex-1 overflow-x-hidden overflow-y-auto rounded-md text-sm focus:outline-none"
            tabindex="-1"
            role="listbox"
          >
            <li
              v-for="(option, index) in options"
              :key="option.value"
              class="hover:bg-default-100 text-foreground relative flex cursor-pointer items-center justify-between rounded-lg px-3 py-2 select-none"
              @click="selectOption(option.value, index)"
              role="option"
              :aria-selected="modelValue === option.value"
            >
              <!-- min-w-0 + flex-1 is the canonical fix for "truncate
                   inside flex doesn't work": in a flex container, items
                   default to min-width: auto (== min-content), so long
                   labels refuse to shrink and overflow horizontally
                   regardless of `truncate`. min-w-0 unsticks that
                   floor so text-overflow:ellipsis actually kicks in. -->
              <span class="block min-w-0 flex-1 truncate">{{ option.label }}</span>
              <KunIcon
                v-if="modelValue === option.value"
                name="lucide:check"
                class="ml-2 shrink-0"
              />
            </li>
          </ul>
        </div>
      </Transition>
    </Teleport>

    <p v-if="error" class="text-danger mt-2 text-sm">{{ error }}</p>
  </div>
</template>
