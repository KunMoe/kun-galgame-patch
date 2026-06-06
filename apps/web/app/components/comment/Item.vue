<script setup lang="ts">
// One comment node — used for BOTH a root (depth 0) and a reply (depth 1).
// It owns the network calls (like / reply / edit / delete) and emits the
// RESULT; the container (comment.vue) applies it to the shared comments array.
// We don't mutate the `comment` prop in place (vue/no-mutate-props) — the
// container holds the source of truth and patches it by id.
//
// One-tier model (matches kungal /galgame/:id): a reply always attaches to the
// ROOT comment (parent_id = rootId), never to another reply, so nesting is at
// most one level. Replying to a reply pre-fills an @mention of that reply's
// author so the "回复 @某人" context survives without a deeper tree.
const props = withDefaults(
  defineProps<{
    comment: PatchPageComment
    galgameId: number
    rootId: number
    depth?: number
    canModerate?: boolean
  }>(),
  { depth: 0, canModerate: false }
)

const emit = defineEmits<{
  liked: [id: number, liked: boolean]
  replyAdded: [reply: PatchPageComment]
  edited: [updated: PatchPageComment]
  removed: [id: number]
}>()

const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const isAuthor = computed(() => userStore.user.id === props.comment.user_id)
// Edit is author-only; delete is author OR moderator (mirrors the backend
// DeleteComment privilege check — patch owner is NOT privileged there).
const canEdit = computed(() => isAuthor.value)
const canDelete = computed(() => isAuthor.value || props.canModerate)
const isEdited = computed(() => !!props.comment.edit)

// ─── Like ──────────────────────────────────────────────
const toggleLike = async () => {
  if (!requireLogin()) return
  const res = await api.put<{ liked: boolean }>(
    `/patch/comment/${props.comment.id}/like`
  )
  if (res.code === 0) emit('liked', props.comment.id, res.data.liked)
  else useKunMessage(res.message || '操作失败', 'error')
}

// ─── Reply ─────────────────────────────────────────────
const replying = ref(false)
const replyContent = ref('')
const replyKey = ref(0) // bump to remount the uncontrolled editor empty
const submittingReply = ref(false)

const openReply = () => {
  if (!requireLogin()) return
  // Replying to a reply (depth 1): seed an @mention so the target is clear,
  // while still attaching to the root (one-tier). markdown renders
  // [@name](/user/id) as a kun-mention link.
  replyContent.value =
    props.depth === 1 && props.comment.user
      ? `[@${props.comment.user.name}](/user/${props.comment.user_id}) `
      : ''
  replyKey.value++
  replying.value = true
}

const submitReply = async () => {
  const text = replyContent.value.trim()
  if (!text) {
    useKunMessage('回复内容不能为空', 'warn')
    return
  }
  submittingReply.value = true
  try {
    const res = await api.post<PatchPageComment>(
      `/patch/${props.galgameId}/comment`,
      { content: text, parent_id: props.rootId }
    )
    if (res.code === 0) {
      replying.value = false
      replyContent.value = ''
      if (res.data?.status === 1) {
        useKunMessage('回复已提交，等待管理员审核通过后显示', 'info')
      } else {
        emit('replyAdded', res.data)
        useKunMessage('回复成功', 'success')
      }
    } else {
      useKunMessage(res.message || '回复失败', 'error')
    }
  } finally {
    submittingReply.value = false
  }
}

// ─── Edit ──────────────────────────────────────────────
const editing = ref(false)
const editContent = ref('')
const editKey = ref(0)
const savingEdit = ref(false)

const startEdit = () => {
  editContent.value = props.comment.content
  editKey.value++
  editing.value = true
}

const submitEdit = async () => {
  const text = editContent.value.trim()
  if (!text) {
    useKunMessage('评论内容不能为空', 'warn')
    return
  }
  if (text === props.comment.content) {
    editing.value = false
    return
  }
  savingEdit.value = true
  try {
    const res = await api.put<PatchPageComment>(
      `/patch/comment/${props.comment.id}`,
      { content: text }
    )
    if (res.code === 0 && res.data) {
      emit('edited', res.data)
      editing.value = false
      useKunMessage('评论已更新', 'success')
    } else {
      useKunMessage(res.message || '更新失败', 'error')
    }
  } finally {
    savingEdit.value = false
  }
}

// ─── Delete ────────────────────────────────────────────
const deleteOpen = ref(false)
const deleting = ref(false)

const confirmDelete = async () => {
  deleting.value = true
  try {
    const res = await api.delete(`/patch/comment/${props.comment.id}`)
    if (res.code === 0) {
      emit('removed', props.comment.id)
      useKunMessage('已删除', 'success')
    } else {
      useKunMessage(res.message || '删除失败', 'error')
    }
  } finally {
    deleting.value = false
    deleteOpen.value = false
  }
}
</script>

<template>
  <div class="flex gap-3">
    <KunAvatar :user="comment.user" :size="depth === 0 ? 'sm' : 'xs'" />

    <div class="min-w-0 flex-1 space-y-1.5">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-sm font-semibold">{{ comment.user.name }}</span>
        <span class="text-default-400 text-xs">
          {{ formatDate(comment.created, { isPrecise: true, isShowYear: true }) }}
        </span>
        <span v-if="isEdited" class="text-default-400 text-xs italic">
          (已编辑)
        </span>
      </div>

      <!-- View vs edit -->
      <KunContent
        v-if="!editing"
        :content="comment.content_html"
        class-name="text-sm"
      />
      <div v-else class="space-y-2">
        <KunMilkdownDualEditorProvider
          :key="`edit-${editKey}`"
          :value-markdown="editContent"
          @set-markdown="(val) => (editContent = val)"
        />
        <div class="flex justify-end gap-2">
          <KunButton variant="light" size="sm" @click="editing = false">
            取消
          </KunButton>
          <KunButton
            color="primary"
            size="sm"
            :loading="savingEdit"
            :disabled="savingEdit"
            @click="submitEdit"
          >
            保存
          </KunButton>
        </div>
      </div>

      <!-- Action row -->
      <div v-if="!editing" class="flex flex-wrap items-center gap-1">
        <KunButton
          :variant="comment.is_liked ? 'flat' : 'light'"
          color="danger"
          size="xs"
          rounded="full"
          :aria-label="comment.is_liked ? '取消点赞' : '点赞'"
          @click="toggleLike"
        >
          <KunIcon
            name="lucide:thumbs-up"
            :class="cn('size-3.5', comment.is_liked && 'fill-current')"
          />
          {{ comment.like_count }}
        </KunButton>

        <KunButton
          variant="light"
          color="default"
          size="xs"
          rounded="full"
          @click="openReply"
        >
          <KunIcon name="lucide:reply" class="size-3.5" />
          回复
        </KunButton>

        <KunButton
          v-if="canEdit"
          variant="light"
          color="default"
          size="xs"
          rounded="full"
          @click="startEdit"
        >
          <KunIcon name="lucide:pencil" class="size-3.5" />
          编辑
        </KunButton>

        <KunButton
          v-if="canDelete"
          variant="light"
          color="danger"
          size="xs"
          rounded="full"
          @click="deleteOpen = true"
        >
          <KunIcon name="lucide:trash-2" class="size-3.5" />
          删除
        </KunButton>
      </div>

      <!-- Reply composer -->
      <div v-if="replying" class="space-y-2 pt-1">
        <KunMilkdownDualEditorProvider
          :key="`reply-${replyKey}`"
          :value-markdown="replyContent"
          @set-markdown="(val) => (replyContent = val)"
        />
        <div class="flex justify-end gap-2">
          <KunButton variant="light" size="sm" @click="replying = false">
            取消
          </KunButton>
          <KunButton
            color="primary"
            size="sm"
            :loading="submittingReply"
            :disabled="submittingReply"
            @click="submitReply"
          >
            <KunIcon name="lucide:send-horizontal" class="size-4" />
            回复
          </KunButton>
        </div>
      </div>

      <!-- Replies (depth-0 only) are rendered by the parent Thread, not here,
           so nesting never exceeds one tier. -->
      <slot name="replies" />
    </div>

    <!-- Delete confirm -->
    <KunModal v-model="deleteOpen" inner-class-name="max-w-md">
      <div class="space-y-4 py-2">
        <h3 class="text-lg font-bold">删除评论？</h3>
        <p class="text-default-600 text-sm">
          此操作不可恢复{{ depth === 0 ? '，该评论下的所有回复也会一并删除' : '' }}。
        </p>
        <div class="flex justify-end gap-2">
          <KunButton variant="light" :disabled="deleting" @click="deleteOpen = false">
            取消
          </KunButton>
          <KunButton
            color="danger"
            :loading="deleting"
            :disabled="deleting"
            @click="confirmDelete"
          >
            确认删除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
