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
const { open: openReport } = useReportModal()

const isAuthor = computed(() => userStore.user.id === props.comment.user_id)
// Edit is author-only; delete is author OR moderator (mirrors the backend
// DeleteComment privilege check — patch owner is NOT privileged there).
const canEdit = computed(() => isAuthor.value)
const canDelete = computed(() => isAuthor.value || props.canModerate)
const isEdited = computed(() => !!props.comment.edit)

// ─── Like ──────────────────────────────────────────────
// KunReaction is an optimistic v-model toggle, so mirror the (parent-owned)
// comment into local refs it can drive, kept in sync if the parent patches the
// comment (e.g. the same comment toggled in the thread drawer).
const liked = ref(props.comment.is_liked)
const likeCount = ref(props.comment.like_count)
watch(() => props.comment.is_liked, (v) => (liked.value = v))
watch(() => props.comment.like_count, (v) => (likeCount.value = v))

const revertLike = (active: boolean) => {
  liked.value = !active
  likeCount.value = Math.max(0, likeCount.value + (active ? -1 : 1))
}

const onLikeChange = async (active: boolean) => {
  if (!requireLogin()) {
    revertLike(active)
    return
  }
  const res = await api.put<{ liked: boolean }>(
    `/patch/comment/${props.comment.id}/like`
  )
  if (res.code === 0) {
    // Propagate to the container so the array (and the drawer's view of the same
    // comment) stays in sync; our local refs already reflect the optimistic state.
    emit('liked', props.comment.id, res.data.liked)
  } else {
    revertLike(active)
    useKunMessage(res.message || '操作失败', 'error')
  }
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
        useKunMessage('回复已提交，等待版主审核通过后显示', 'info')
      } else {
        // Same as the root composer: the create response omits the resolved
        // `user`; the replier is the current user, so stamp it before emitting.
        emit('replyAdded', { ...res.data, user: userStore.user })
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
const deleteReason = ref('')
// A moderator deleting SOMEONE ELSE'S comment → offer a reason, recorded in the
// author's notification + the admin audit log. Author self-deletes need none.
const isForeignDelete = computed(() => !isAuthor.value)
const askDelete = () => {
  deleteReason.value = ''
  deleteOpen.value = true
}

const confirmDelete = async () => {
  deleting.value = true
  try {
    const res = await api.delete(
      `/patch/comment/${props.comment.id}`,
      isForeignDelete.value ? { reason: deleteReason.value.trim() } : undefined
    )
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

// ─── Actions menu (⋮) ──────────────────────────────────
// Edit + delete live in a top-right "⋮" dropdown (not inline buttons), keeping
// the action row to 点赞 / 回复. Shape mirrors resource.vue's KunDropdown items.
interface CommentMenuItem {
  key: 'edit' | 'delete' | 'report'
  label: string
  icon: string
  color?:
    | 'default'
    | 'primary'
    | 'secondary'
    | 'success'
    | 'warning'
    | 'danger'
    | 'info'
}
const menuItems = computed<CommentMenuItem[]>(() => {
  const items: CommentMenuItem[] = []
  if (canEdit.value) {
    items.push({ key: 'edit', label: '编辑', icon: 'lucide:pencil' })
  }
  if (canDelete.value) {
    items.push({ key: 'delete', label: '删除', icon: 'lucide:trash-2', color: 'danger' })
  }
  // Report is offered to everyone EXCEPT the author (you don't report yourself).
  if (!isAuthor.value) {
    items.push({ key: 'report', label: '举报', icon: 'lucide:flag', color: 'danger' })
  }
  return items
})
const onMenuSelect = (item: { key: string }) => {
  if (item.key === 'edit') startEdit()
  else if (item.key === 'delete') askDelete()
  else if (item.key === 'report') reportComment()
}
// Report this comment → global report modal (patch_comment). Snapshot the
// content as evidence; patch_comment has no standalone page, so no subject_url.
const reportComment = () => {
  if (!requireLogin()) return
  openReport({
    subjectKind: 'patch_comment',
    subjectId: props.comment.id,
    snapshot: props.comment.content
  })
}
</script>

<template>
  <!-- Anchor for deep-linking from messages / home / global comments
       (/patch/:id/comment#comment-:id). scroll-mt keeps the target clear of the
       sticky patch header when scrolled into view. -->
  <div :id="`comment-${comment.id}`" class="flex scroll-mt-24 gap-3">
    <KunAvatar :user="comment.user" :size="depth === 0 ? 'sm' : 'xs'" />

    <div class="min-w-0 flex-1 space-y-1.5">
      <div class="flex items-start justify-between gap-2">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <span class="text-sm font-semibold">{{ comment.user.name }}</span>
          <span class="text-default-400 text-xs">
            {{ formatDate(comment.created, { isPrecise: true, isShowYear: true }) }}
          </span>
          <span v-if="isEdited" class="text-default-400 text-xs italic">
            (已编辑)
          </span>
        </div>

        <!-- ⋮ menu — edit / delete (top-right). Only when the viewer can do
             either and isn't already editing; keeps the action row to 点赞/回复. -->
        <KunDropdown
          v-if="!editing && menuItems.length"
          :items="menuItems"
          position="bottom-end"
          @select="onMenuSelect"
        >
          <template #trigger>
            <KunButton
              is-icon-only
              variant="light"
              color="default"
              size="xs"
              rounded="full"
              class-name="text-default-400 -mr-1 shrink-0"
              aria-label="更多操作"
            >
              <KunIcon name="lucide:ellipsis-vertical" class="size-4" />
            </KunButton>
          </template>
        </KunDropdown>
      </div>

      <!-- View vs edit -->
      <KunContent v-if="!editing" compact :content="comment.content_html" />
      <div v-else class="space-y-2">
        <KunMarkdownEditor
          :key="`edit-${editKey}`"
          :model-value="editContent"
          @update:model-value="(val) => (editContent = val)"
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
        <KunReaction
          v-model="liked"
          v-model:count="likeCount"
          icon="lucide:thumbs-up"
          color="primary"
          size="sm"
          label="点赞"
          @change="onLikeChange"
        />

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
      </div>

      <!-- Reply composer -->
      <div v-if="replying" class="space-y-2 pt-1">
        <KunMarkdownEditor
          :key="`reply-${replyKey}`"
          :model-value="replyContent"
          @update:model-value="(val) => (replyContent = val)"
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
        <div v-if="isForeignDelete" class="space-y-1">
          <label class="text-default-600 text-sm">
            删除原因（可选，会通知作者并记入管理日志）
          </label>
          <KunInput
            v-model="deleteReason"
            placeholder="例如：垃圾广告 / 人身攻击 / 违规内容"
          />
        </div>
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
