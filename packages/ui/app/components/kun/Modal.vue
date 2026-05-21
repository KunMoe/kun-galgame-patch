<script setup lang="ts">
import { useEventListener } from '@vueuse/core'
import { onMounted, onUnmounted, watch } from 'vue'
import { useBodyScrollLock } from '../../composables/useBodyScrollLock'

const props = withDefaults(
  defineProps<{
    className?: string
    innerClassName?: string
    isDismissable?: boolean
    isShowCloseButton?: boolean
    withContainer?: boolean
  }>(),
  {
    className: '',
    innerClassName: '',
    isDismissable: true,
    isShowCloseButton: true,
    withContainer: true
  }
)

const modelValue = defineModel<boolean>({ required: true })

const emits = defineEmits<{
  close: []
}>()

// The singleton lock counter lives in useBodyScrollLock; `locked` here
// is per-instance and guarantees onUnmounted releases exactly once
// regardless of how many times modelValue toggled.
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

const handleCloseKunModal = () => {
  if (props.isDismissable) {
    modelValue.value = false
    emits('close')
  }
}

useEventListener('keydown', (e: KeyboardEvent) => {
  if (e.key === 'Escape' && modelValue.value) {
    handleCloseKunModal()
  }
})

watch(modelValue, (v) => applyLock(v))

onMounted(() => {
  if (modelValue.value) applyLock(true)
})

onUnmounted(() => {
  applyLock(false)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="kun-modal">
      <div
        v-if="modelValue"
        :class="
          cn(
            'bg-default-800/70 dark:bg-background/70 fixed top-0 left-0 z-1007 flex h-full w-full items-center justify-center p-3 transition-all',
            className
          )
        "
        @click="handleCloseKunModal"
        tabindex="0"
      >
        <div
          v-if="withContainer"
          :class="
            cn(
              'bg-content1/85 scrollbar-hide relative m-auto max-h-[90vh] min-w-80 overflow-y-auto rounded-lg border p-6 backdrop-blur-[var(--kun-background-blur)] transition-all',
              innerClassName
            )
          "
          @click.stop
        >
          <slot />

          <KunButton
            v-if="isShowCloseButton"
            color="default"
            variant="light"
            class-name="absolute top-1 right-1"
            rounded="full"
            :is-icon-only="true"
            @click="
              () => {
                modelValue = false
                emits('close')
              }
            "
          >
            <KunIcon class="icon" name="lucide:x" />
          </KunButton>
        </div>

        <slot v-else />
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.kun-modal-enter-active,
.kun-modal-leave-active {
  transition: all 0.3s ease;
}

.kun-modal-enter-from {
  opacity: 0;
  transform: scale(1.1);
}

.kun-modal-leave-to {
  opacity: 0;
  transform: scale(1.1);
}
</style>
