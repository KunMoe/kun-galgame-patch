<script setup lang="ts">
import { commentSurface, type CommentTarget } from '~/shared/utils/commentTarget'

const props = withDefaults(
  defineProps<{
    target: CommentTarget
    replyToPostId?: number | null
    seed?: string
    isReply?: boolean
  }>(),
  { replyToPostId: null, seed: '', isReply: false }
)

const emit = defineEmits<{
  close: []
  submitted: [comment: PatchPageComment]
}>()

const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const surface = commentSurface(props.target)

const content = ref(props.seed)
const publishing = ref(false)
const editorKey = ref(0)

const resetToSeed = () => {
  content.value = props.seed
  editorKey.value++
}

watch(
  () => props.seed,
  (next) => {
    if (!content.value.trim()) {
      content.value = next
      editorKey.value++
    }
  }
)

const publish = async () => {
  if (!requireLogin()) return
  const text = content.value.trim()
  if (!text) {
    useKunMessage(props.isReply ? '回复内容不能为空' : '评论内容不能为空', 'warn')
    return
  }

  publishing.value = true
  try {
    const res = await api.post<PatchPageComment>(surface.createUrl, {
      content: text,
      ...(props.replyToPostId ? { reply_to_post_id: props.replyToPostId } : {})
    })
    if (res.code !== 0) {
      useKunMessage(res.message || '发布失败', 'error')
      return
    }
    resetToSeed()

    // A newcomer's first posts are HELD upstream: created hidden and queued for
    // review. They are still returned to their own author, so the row renders
    // with its 审核中 label rather than vanishing — but say so here too, or the
    // post looks like it simply failed and gets written again.
    if (res.data?.held) {
      useKunMessage(
        props.isReply
          ? '回复已提交，新用户的前两条内容会先经过审核'
          : '评论已提交，新用户的前两条内容会先经过审核',
        'info'
      )
    } else {
      useKunMessage(props.isReply ? '回复成功' : '评论发布成功', 'success')
    }
    emit('submitted', { ...res.data, user: res.data.user ?? userStore.user })
    emit('close')
  } finally {
    publishing.value = false
  }
}
</script>

<template>
  <div class="space-y-3">
    <KunMarkdownEditor
      :key="editorKey"
      :model-value="content"
      :placeholder="isReply ? '写下你的回复～' : surface.composerPlaceholder"
      @update:model-value="(val) => (content = val)"
    />

    <div class="flex justify-end gap-2">
      <KunButton v-if="isReply" variant="light" size="sm" @click="emit('close')">
        取消
      </KunButton>
      <KunButton
        color="primary"
        rounded="full"
        :size="isReply ? 'sm' : 'md'"
        :loading="publishing"
        :disabled="publishing"
        @click="publish"
      >
        <KunIcon name="lucide:send-horizontal" class="size-4" />
        {{ isReply ? '发布回复' : '发布评论' }}
      </KunButton>
    </div>
  </div>
</template>
