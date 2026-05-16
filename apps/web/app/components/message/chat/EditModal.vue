<script setup lang="ts">
// Ported from next-web EditMessageModal.tsx. Edits the raw markdown
// (message.content); the backend re-renders and flips status to EDITED.
const props = defineProps<{
  open: boolean
  initial: string
}>()

const emit = defineEmits<{
  'update:open': [v: boolean]
  save: [content: string]
}>()

const draft = ref(props.initial)
watch(
  () => props.open,
  (o) => {
    if (o) draft.value = props.initial
  }
)

const close = () => emit('update:open', false)
const save = () => {
  const v = draft.value.trim()
  if (!v) return
  emit('save', v)
}
</script>

<template>
  <KunModal
    :modal-value="open"
    inner-class-name="max-w-lg"
    @update:modal-value="(v) => emit('update:open', v)"
  >
    <div class="space-y-4">
      <div class="space-y-1">
        <h3 class="text-lg font-semibold">重新编辑消息</h3>
        <p class="text-default-500 text-sm">
          系统不会显示您编辑之前的消息，但消息时间前会增加「已编辑」提示。
        </p>
      </div>
      <textarea
        v-model="draft"
        rows="4"
        autofocus
        class="border-default/20 bg-background w-full rounded-lg border p-3 text-sm"
        placeholder="支持 Markdown"
      />
      <div class="flex justify-end gap-2">
        <KunButton variant="light" color="danger" @click="close">
          取消
        </KunButton>
        <KunButton color="primary" :disabled="!draft.trim()" @click="save">
          保存
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
