<script setup lang="ts">
const props = defineProps<{ conversation: ChatConversation }>()

const act = useChatActions()
const busy = ref(false)
const popover = ref<{ close: () => void } | null>(null)
const clearOpen = ref(false)

const muted = computed(() => chatMuted(props.conversation))
const pinned = computed(() => props.conversation.me.pinned_rank !== null)
const archived = computed(() => props.conversation.me.archived)

const run = async (patch: Record<string, unknown>) => {
  popover.value?.close()
  busy.value = true
  const r = await act.setDialog(props.conversation.id, patch)
  busy.value = false
  if (r.code !== 0) useKunMessage(r.message, 'warn')
}

const askClear = () => {
  popover.value?.close()
  clearOpen.value = true
}

const clear = async () => {
  busy.value = true
  const r = await act.clearHistory(props.conversation.id, false)
  busy.value = false
  clearOpen.value = false
  if (r.code !== 0) useKunMessage(r.message, 'warn')
}

const itemClass =
  'hover:bg-default-100 rounded-kun-md flex w-full items-center gap-2 px-2 py-2 text-left text-sm'
</script>

<template>
  <KunPopover ref="popover" position="bottom-end" inner-class="p-1.5 min-w-40">
    <template #trigger>
      <KunButton
        variant="light"
        size="sm"
        is-icon-only
        aria-label="对话设置"
        :loading="busy"
      >
        <KunIcon name="lucide:ellipsis-vertical" class="size-5" />
      </KunButton>
    </template>
    <div class="flex flex-col">
      <button
        v-if="props.conversation.me.accepted"
        type="button"
        :class="itemClass"
        @click="run({ pinned: !pinned })"
      >
        <KunIcon
          :name="pinned ? 'lucide:pin-off' : 'lucide:pin'"
          class="size-4"
        />
        {{ pinned ? '取消置顶' : '置顶对话' }}
      </button>
      <button type="button" :class="itemClass" @click="run({ muted: !muted })">
        <KunIcon
          :name="muted ? 'lucide:bell' : 'lucide:bell-off'"
          class="size-4"
        />
        {{ muted ? '取消静音' : '静音' }}
      </button>
      <button
        v-if="props.conversation.me.accepted"
        type="button"
        :class="itemClass"
        @click="run({ archived: !archived })"
      >
        <KunIcon
          :name="archived ? 'lucide:archive-restore' : 'lucide:archive'"
          class="size-4"
        />
        {{ archived ? '移出归档' : '归档' }}
      </button>
      <button
        type="button"
        :class="itemClass"
        @click="run({ marked_unread: true })"
      >
        <KunIcon name="lucide:mail" class="size-4" />标为未读
      </button>
      <button
        type="button"
        :class="cn(itemClass, 'text-danger')"
        @click="askClear"
      >
        <KunIcon name="lucide:eraser" class="size-4" />清空聊天记录
      </button>
    </div>
  </KunPopover>

  <KunModal
    v-model="clearOpen"
    title="清空聊天记录"
    size="sm"
    is-show-close-button
  >
    <div class="flex flex-col gap-4 p-1">
      <p class="text-default-600 text-sm">
        只会清空你这边的聊天记录，对方仍能看到。
      </p>
      <div class="flex justify-end gap-2">
        <KunButton variant="light" @click="clearOpen = false">取消</KunButton>
        <KunButton color="danger" :loading="busy" @click="clear"
          >清空</KunButton
        >
      </div>
    </div>
  </KunModal>
</template>
