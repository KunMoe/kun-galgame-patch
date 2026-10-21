<script setup lang="ts">
const props = defineProps<{ deletion: WorkDeletion }>()

const reason = computed(() =>
  workDeletionReasonText(props.deletion.deleted_reason)
)
const note = computed(() => {
  const restore = '如果之后恢复，本页会重新显示。'
  const at = props.deletion.deleted_at
  return at
    ? `删除于 ${formatDate(at, { isShowYear: true })}，${restore}`
    : restore
})
</script>

<template>
  <div
    class="mx-auto flex w-full max-w-xl flex-col items-center gap-6 px-4 py-16 text-center"
  >
    <div
      class="bg-default-100 text-default-500 flex size-16 items-center justify-center rounded-full"
    >
      <KunIcon name="lucide:archive" class="size-8" />
    </div>
    <div class="space-y-2">
      <h1 class="text-2xl font-bold">这部作品已被删除</h1>
      <p v-if="reason" class="text-default-600 text-sm">
        删除原因：{{ reason }}
      </p>
      <p class="text-default-500 text-sm leading-relaxed">{{ note }}</p>
    </div>
    <NuxtLink to="/">
      <KunButton variant="light" color="default" size="md">
        返回首页
      </KunButton>
    </NuxtLink>
  </div>
</template>
