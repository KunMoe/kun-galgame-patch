<script setup lang="ts">
import {
  commentAbsoluteUrl,
  commentAnchorId,
  commentSurface,
  type CommentTarget
} from '~/shared/utils/commentTarget'

const props = withDefaults(
  defineProps<{
    comment: PatchPageComment
    target: CommentTarget
    depth?: number
    expanded?: boolean
    // Loaded replies the collapsed view leaves out.
    hiddenReplyCount?: number
    // Visible replies not loaded yet.
    remainingReplyCount?: number
    // More replies are loaded than the collapsed view shows.
    collapsible?: boolean
    loadingReplies?: boolean
  }>(),
  {
    depth: 0,
    expanded: false,
    hiddenReplyCount: 0,
    remainingReplyCount: 0,
    collapsible: false,
    loadingReplies: false
  }
)

const emit = defineEmits<{
  liked: [id: number, liked: boolean]
  replyAdded: [reply: PatchPageComment]
  edited: [updated: PatchPageComment]
  removed: [id: number, source: CommentDeleteSource]
  restored: [updated: PatchPageComment]
  pinned: [updated: PatchPageComment]
  toggleExpand: [rootId: number]
  loadReplies: [rootId: number]
}>()

const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const surface = commentSurface(props.target)

const isAuthor = computed(() => userStore.user.id === props.comment.user?.id)
const { can } = usePermissions()
const canEdit = computed(() => isAuthor.value || can('comment.edit_any'))
const canDelete = computed(() => isAuthor.value || can('comment.delete_any'))
const isEdited = computed(() => !!props.comment.edited)
const canPin = computed(
  () => props.depth === 0 && !props.comment.held && can('comment.pin')
)
const canRestore = computed(
  () =>
    can('comment.delete_any') &&
    (props.comment.delete_source === 'moderator' ||
      props.comment.delete_source === 'review')
)

const TOMBSTONE: Record<CommentDeleteSource, string> = {
  author: '该评论已被作者删除',
  moderator: '该评论已被管理员删除',
  review: '该评论因违反社区规范已被移除',
  trust: '该评论因违反社区规范已被移除',
  purge: '该评论已删除'
}
const tombstone = computed(() =>
  props.comment.delete_source
    ? TOMBSTONE[props.comment.delete_source]
    : '该评论已删除'
)

const moreReplyCount = computed(
  () => props.hiddenReplyCount + props.remainingReplyCount
)
const showReplyBar = computed(
  () =>
    props.depth === 0 &&
    (props.expanded
      ? props.remainingReplyCount > 0 || props.collapsible
      : moreReplyCount.value > 0)
)

const expandReplies = () => {
  emit('toggleExpand', props.comment.id)
  if (props.hiddenReplyCount === 0 && props.remainingReplyCount > 0) {
    emit('loadReplies', props.comment.id)
  }
}

const pinning = ref(false)
const togglePin = async () => {
  pinning.value = true
  try {
    const url = `/patch/comment/${props.comment.id}/pin`
    const res = props.comment.pinned_at
      ? await api.delete<PatchPageComment>(url)
      : await api.put<PatchPageComment>(url)
    if (res.code === 0 && res.data) {
      emit('pinned', res.data)
      useKunMessage(res.data.pinned_at ? '已置顶' : '已取消置顶', 'success')
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    pinning.value = false
  }
}

const restoring = ref(false)
const restore = async () => {
  restoring.value = true
  try {
    const res = await api.post<PatchPageComment>(
      `/patch/comment/${props.comment.id}/restore`
    )
    if (res.code === 0 && res.data) {
      emit('restored', res.data)
      useKunMessage(
        res.data.held ? '评论已恢复，仍在审核中，仅作者可见' : '评论已恢复',
        'success'
      )
    } else {
      useKunMessage(res.message || '恢复失败', 'error')
    }
  } finally {
    restoring.value = false
  }
}

const liked = ref(props.comment.is_liked)
const likeCount = ref(props.comment.like_count)
watch(
  () => props.comment.is_liked,
  (v) => (liked.value = v)
)
watch(
  () => props.comment.like_count,
  (v) => (likeCount.value = v)
)

const revertLike = (active: boolean) => {
  liked.value = !active
  likeCount.value = Math.max(0, likeCount.value + (active ? -1 : 1))
}

const onLikeChange = async (active: boolean) => {
  if (!requireLogin()) {
    revertLike(active)
    return
  }
  const url = `/patch/comment/${props.comment.id}/like`
  const res = active
    ? await api.put<{ liked: boolean; like_count: number }>(url)
    : await api.delete<{ liked: boolean; like_count: number }>(url)
  if (res.code === 0) {
    emit('liked', props.comment.id, res.data.liked)
  } else {
    revertLike(active)
    useKunMessage(res.message || '操作失败', 'error')
  }
}

const replying = ref(false)
const replySeed = ref('')

const openReply = () => {
  if (!requireLogin()) return
  replySeed.value =
    props.depth === 1 && props.comment.user
      ? `[@${props.comment.user.name}](/user/${props.comment.user.id}) `
      : ''
  replying.value = true
}

// A reply always attaches to the ROOT post — moyu's wall is two tiers, and
// community threads a reply-to-a-reply under the same root.
const replyToId = computed(() =>
  props.depth === 1
    ? (props.comment.root_comment_id ?? props.comment.id)
    : props.comment.id
)

const onReplySubmitted = (reply: PatchPageComment) => {
  replying.value = false
  emit('replyAdded', reply)
}

const editing = ref(false)
const editContent = ref('')
const editReason = ref('')
const editKey = ref(0)
const savingEdit = ref(false)

// The editor is seeded from the post's markdown source, which the wall read
// already carries — no second request to open it.
const startEdit = () => {
  editContent.value = props.comment.content
  editReason.value = ''
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
      isAuthor.value
        ? { content: text }
        : { content: text, reason: editReason.value.trim() }
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

const deleteOpen = ref(false)
const deleting = ref(false)
const deleteReason = ref('')

const askDelete = () => {
  deleteReason.value = ''
  deleteOpen.value = true
}

const confirmDelete = async () => {
  deleting.value = true
  try {
    const res = await api.delete(
      `/patch/comment/${props.comment.id}`,
      isAuthor.value ? undefined : { reason: deleteReason.value.trim() }
    )
    if (res.code === 0) {
      emit('removed', props.comment.id, isAuthor.value ? 'author' : 'moderator')
      useKunMessage('已删除', 'success')
    } else {
      useKunMessage(res.message || '删除失败', 'error')
    }
  } finally {
    deleting.value = false
    deleteOpen.value = false
  }
}

const reportOpen = ref(false)

const reportComment = () => {
  if (!requireLogin()) return
  reportOpen.value = true
}
</script>

<template>
  <div :id="commentAnchorId(comment.id)" class="flex scroll-mt-24 gap-3">
    <KunAvatar
      :user="toKunUser(comment.user)"
      :size="depth === 0 ? 'md' : 'sm'"
    />

    <div class="min-w-0 flex-1">
      <div
        class="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-xs leading-5"
      >
        <span class="text-default-800 text-sm font-medium">
          {{ comment.user?.name ?? '已注销用户' }}
        </span>
        <span v-if="comment.target_user" class="text-default-400">
          回复
          <span class="text-primary-500">@{{ comment.target_user.name }}</span>
        </span>
        <span class="text-default-400">
          {{
            formatDate(comment.created, { isPrecise: true, isShowYear: true })
          }}
        </span>
        <KunChip
          v-if="depth === 0 && comment.pinned_at"
          color="primary"
          size="xs"
          variant="flat"
        >
          <KunIcon name="lucide:pin" class="size-3" />
          置顶
        </KunChip>
        <span v-if="isEdited" class="text-default-400 italic">
          {{ comment.edited_by_moderator ? '已编辑（管理）' : '已编辑' }}
        </span>
        <!--
          Held by the newcomer sandbox: created hidden and queued for review.
          Only its own author is served this row at all, and without the label
          they would think the comment simply failed and post it again.
        -->
        <span
          v-if="comment.held"
          class="bg-warning/15 text-warning rounded-full px-2 py-0.5 text-xs"
        >
          审核中，仅你可见
        </span>
      </div>

      <div
        v-if="comment.deleted"
        class="mt-2 flex flex-wrap items-center gap-2 text-sm"
      >
        <span class="text-default-400 italic">{{ tombstone }}</span>
        <KunButton
          v-if="canRestore"
          variant="light"
          color="primary"
          size="xs"
          :loading="restoring"
          :disabled="restoring"
          @click="restore"
        >
          <KunIcon name="lucide:undo-2" class="size-3.5" />
          恢复评论
        </KunButton>
      </div>
      <KunContent
        v-else-if="!editing"
        class="mt-2"
        compact
        :content="comment.content_html"
      />
      <div v-else class="mt-2 space-y-2">
        <p v-if="!isAuthor" class="text-warning text-xs">
          正在以管理身份编辑他人的评论，保存后会标注「已编辑（管理）」
        </p>
        <KunMarkdownEditor
          :key="`edit-${editKey}`"
          :model-value="editContent"
          @update:model-value="(val) => (editContent = val)"
        />
        <div v-if="!isAuthor" class="space-y-1">
          <label class="text-default-600 text-sm">
            编辑原因（可选，会通知作者并记入管理日志）
          </label>
          <KunInput
            v-model="editReason"
            placeholder="例如：移除广告链接 / 删去人身攻击"
          />
        </div>
        <div class="flex justify-end gap-2">
          <KunButton
            variant="light"
            color="default"
            size="sm"
            @click="editing = false"
          >
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

      <div
        v-if="!editing && !comment.deleted"
        class="mt-2.5 flex items-center gap-1"
      >
        <KunTooltip text="回复">
          <KunReaction
            :toggle="false"
            size="sm"
            icon="lucide:reply"
            label="回复"
            @click="openReply"
          />
        </KunTooltip>

        <KunTooltip text="点赞">
          <KunReaction
            v-model="liked"
            v-model:count="likeCount"
            size="sm"
            icon="lucide:thumbs-up"
            color="primary"
            label="点赞"
            @change="onLikeChange"
          />
        </KunTooltip>

        <KunPopover position="bottom-start">
          <template #trigger>
            <KunReaction
              :toggle="false"
              size="sm"
              icon="lucide:ellipsis"
              label="更多"
            />
          </template>

          <div class="flex w-44 flex-col gap-2 p-2">
            <KunButton
              v-if="canPin"
              variant="light"
              color="default"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              :loading="pinning"
              :disabled="pinning"
              @click="togglePin"
            >
              <KunIcon
                :name="comment.pinned_at ? 'lucide:pin-off' : 'lucide:pin'"
                class="size-4"
              />
              {{ comment.pinned_at ? '取消置顶' : '置顶评论' }}
            </KunButton>

            <KunButton
              v-if="canEdit"
              variant="light"
              color="default"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              @click="startEdit"
            >
              <KunIcon name="lucide:pencil" class="size-4" />
              编辑评论
            </KunButton>

            <KunButton
              v-if="!isAuthor"
              variant="light"
              color="danger"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              @click="reportComment"
            >
              <KunIcon name="lucide:flag" class="size-4" />
              举报评论
            </KunButton>

            <KunButton
              v-if="canDelete"
              variant="light"
              color="danger"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              @click="askDelete"
            >
              <KunIcon name="lucide:trash-2" class="size-4" />
              删除评论
            </KunButton>
          </div>
        </KunPopover>
      </div>

      <KunFadeCard>
        <CommentComposer
          v-if="replying"
          class="mt-3"
          :target="target"
          :reply-to-post-id="replyToId"
          :seed="replySeed"
          is-reply
          @close="replying = false"
          @submitted="onReplySubmitted"
        />
      </KunFadeCard>

      <slot name="replies" />

      <div v-if="showReplyBar" class="mt-3 flex flex-wrap gap-1">
        <KunButton
          v-if="!expanded && moreReplyCount > 0"
          variant="light"
          color="primary"
          size="sm"
          :loading="loadingReplies"
          @click="expandReplies"
        >
          <KunIcon name="lucide:chevron-down" class="size-4" />
          展开更多 {{ moreReplyCount }} 条回复
        </KunButton>
        <KunButton
          v-if="expanded && remainingReplyCount > 0"
          variant="light"
          color="primary"
          size="sm"
          :loading="loadingReplies"
          :disabled="loadingReplies"
          @click="emit('loadReplies', comment.id)"
        >
          <KunIcon name="lucide:chevron-down" class="size-4" />
          加载更多回复（还有 {{ remainingReplyCount }} 条）
        </KunButton>
        <KunButton
          v-if="expanded && collapsible"
          variant="light"
          color="default"
          size="sm"
          @click="emit('toggleExpand', comment.id)"
        >
          <KunIcon name="lucide:chevron-up" class="size-4" />
          收起回复
        </KunButton>
      </div>
    </div>

    <CommentFlagModal
      v-model="reportOpen"
      :post-id="comment.id"
      :evidence-url="commentAbsoluteUrl(surface, comment.id)"
    />

    <KunModal
      v-model="deleteOpen"
      inner-class-name="max-w-md"
      aria-label="删除评论"
    >
      <div class="space-y-4 py-2">
        <h3 class="text-lg font-bold">删除评论？</h3>
        <!--
          The reply tier is NOT deleted with its root. Community tombstones the
          one post and keeps its number, which is what stops a wall's numbering
          from collapsing under it.
        -->
        <p class="text-default-600 text-sm">
          {{
            isAuthor
              ? '此操作不可恢复，该评论会保留位置并显示为「已删除」。'
              : '该评论会保留位置并显示为「已被管理员删除」，之后仍可由管理员恢复。'
          }}
        </p>
        <div v-if="!isAuthor" class="space-y-1">
          <label class="text-default-600 text-sm">
            删除原因（可选，会通知作者并记入管理日志）
          </label>
          <KunInput
            v-model="deleteReason"
            placeholder="例如：垃圾广告 / 人身攻击 / 违规内容"
          />
        </div>
        <div class="flex justify-end gap-2">
          <KunButton
            variant="light"
            :disabled="deleting"
            @click="deleteOpen = false"
          >
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
