<script setup lang="ts">
import { COMMENT_FLAG_REASONS } from '~/shared/utils/commentTarget'

const props = defineProps<{
  postId: number
  // The comment's absolute URL, so a moderator opens it in context.
  evidenceUrl: string
}>()

const open = defineModel<boolean>({ required: true })

const api = useApi()

const reason = ref<number>(COMMENT_FLAG_REASONS[0].value)
const note = ref('')
const submitting = ref(false)

const submit = async () => {
  submitting.value = true
  try {
    const res = await api.post(`/patch/comment/${props.postId}/flag`, {
      reason: reason.value,
      note: [note.value.trim(), props.evidenceUrl].filter(Boolean).join('\n')
    })
    if (res.code === 0) {
      useKunMessage('举报已提交，感谢你的反馈', 'success')
      open.value = false
      note.value = ''
    } else {
      useKunMessage(res.message || '举报失败', 'error')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-md" aria-label="举报评论">
    <div class="space-y-4 py-2">
      <h3 class="text-lg font-bold">举报这条评论</h3>
      <!--
        The report's weight is the reporter's, not the button's: the community
        service multiplies their trust level by how often their past reports were
        upheld, and enough weight hides the comment and opens a review item.
      -->
      <p class="text-default-600 text-sm">
        举报会交给审核队列处理。重复的无效举报会降低你后续举报的权重，请如实填写。
      </p>

      <div class="flex flex-wrap gap-2">
        <KunButton
          v-for="r in COMMENT_FLAG_REASONS"
          :key="r.value"
          size="sm"
          variant="light"
          :color="reason === r.value ? 'primary' : 'default'"
          @click="reason = r.value"
        >
          {{ r.label }}
        </KunButton>
      </div>

      <KunInput v-model="note" placeholder="补充说明（可选）" />

      <div class="flex justify-end gap-2">
        <KunButton variant="light" :disabled="submitting" @click="open = false">
          取消
        </KunButton>
        <KunButton
          color="danger"
          :loading="submitting"
          :disabled="submitting"
          @click="submit"
        >
          提交举报
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
