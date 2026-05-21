<script setup lang="ts">
import { computed, watch } from 'vue'
import type { KunFileInputProps } from './type'

const props = withDefaults(defineProps<KunFileInputProps>(), {
  accept: '',
  multiple: false,
  maxSize: undefined,
  hint: '',
  error: '',
  disabled: false,
  triggerText: '选择文件',
  triggerIcon: 'lucide:upload',
  triggerVariant: 'flat',
  triggerColor: 'primary',
  triggerSize: 'md',
  fullWidth: false,
  showFileName: true,
  className: ''
})

// Two named v-models — convention: use the one that matches `multiple`.
//   single mode (default): v-model="bannerFile"        File | null
//   multi  mode:           v-model:files="screenshots" File[]
// Two models keep the consumer-side TS narrow without a discriminated
// union; the unused model just stays at its default.
const file = defineModel<File | null>({ default: null })
const filesModel = defineModel<File[]>('files', { default: () => [] })

const emit = defineEmits<{
  // Always an array regardless of multiple mode — single mode emits
  // [file], multi mode emits [f1, f2, ...]. Lets consumers `forEach`
  // or `next[0]` uniformly without isArray branching.
  change: [picked: File[]]
  errorPick: [message: string]
}>()

// Delegate selection mechanics to useFilePicker. Component is a thin
// declarative wrapper around the composable — no duplicated logic.
const { pickFiles, files: pickedFiles } = useFilePicker({
  accept: props.accept || undefined,
  multiple: props.multiple,
  maxSize: props.maxSize,
  onError: (msg) => emit('errorPick', msg)
})

watch(pickedFiles, (next) => {
  // User cancelled the dialog → keep previous selection (matches the
  // native <input type="file"> cancel-doesn't-clear behavior).
  if (next.length === 0) {
    return
  }
  if (props.multiple) {
    filesModel.value = next
  } else {
    file.value = next[0] ?? null
  }
  emit('change', next)
})

const displayName = computed<string | null>(() => {
  if (props.multiple) {
    const n = filesModel.value.length
    return n === 0 ? null : `已选 ${n} 个文件`
  }
  return file.value?.name ?? null
})

const handlePick = () => {
  if (props.disabled) {
    return
  }
  pickFiles()
}
</script>

<template>
  <div :class="cn('flex flex-col gap-1', className)">
    <div class="flex items-center gap-2">
      <!-- Custom trigger via default slot; gets pick / fileName /
           disabled as slot props for full control. Falls back to a
           KunButton built from trigger* props. -->
      <slot
        :pick="handlePick"
        :disabled="disabled"
        :file-name="displayName"
      >
        <KunButton
          :variant="triggerVariant"
          :color="triggerColor"
          :size="triggerSize"
          :disabled="disabled"
          :full-width="fullWidth"
          type="button"
          @click="handlePick"
        >
          <Icon
            v-if="triggerIcon"
            :name="triggerIcon"
            class="mr-1 size-4"
          />
          {{ triggerText }}
        </KunButton>
      </slot>
      <span
        v-if="showFileName && displayName"
        class="text-default-500 truncate text-sm"
      >
        {{ displayName }}
      </span>
    </div>
    <p v-if="hint && !error" class="text-default-400 text-xs">
      {{ hint }}
    </p>
    <p v-if="error" class="text-danger text-xs">{{ error }}</p>
  </div>
</template>
