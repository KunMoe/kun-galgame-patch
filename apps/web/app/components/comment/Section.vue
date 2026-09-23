<script setup lang="ts">
import type { CommentGroup } from '~/composables/useCommentList'
import type { CommentTarget } from '~/shared/utils/commentTarget'
import { commentSurface } from '~/shared/utils/commentTarget'

const props = withDefaults(
  defineProps<{
    target: CommentTarget
    groups: CommentGroup[]
    expandedRoots: Set<number>
    subscription: CommentWallState | null
    hasMore?: boolean
    loadingMore?: boolean
    pending?: boolean
    canModerate?: boolean
    mentionUser?: KunUser | null
  }>(),
  {
    hasMore: false,
    loadingMore: false,
    pending: false,
    canModerate: false,
    mentionUser: null
  }
)

const emit = defineEmits<{
  commentAdded: [comment: PatchPageComment]
  liked: [id: number, liked: boolean]
  edited: [updated: PatchPageComment]
  removed: [id: number]
  toggleExpand: [rootId: number]
  loadMore: []
  setLevel: [level: CommentNotificationLevel]
}>()

const userStore = useUserStore()
const surface = commentSurface(props.target)

const INLINE_LIMIT = 3
const visibleReplies = (group: CommentGroup) =>
  props.expandedRoots.has(group.root.id)
    ? group.replies
    : group.replies.slice(0, INLINE_LIMIT)

const hiddenReplyCount = (group: CommentGroup) =>
  Math.max(0, group.replies.length - INLINE_LIMIT)

const composerSeed = computed(() => {
  const u = props.mentionUser
  if (!u?.id || u.id === userStore.user.id) return ''
  return `[@${u.name}](/user/${u.id}) `
})
</script>

<template>
  <div class="space-y-6">
    <div
      v-if="surface.notice"
      class="border-primary/30 bg-primary/10 flex gap-3 rounded-2xl border p-4"
    >
      <KunIcon
        name="lucide:megaphone"
        class="text-primary mt-0.5 size-5 shrink-0"
      />
      <div class="min-w-0 space-y-1">
        <p class="text-primary text-sm font-semibold">
          {{ surface.notice.title }}
        </p>
        <p class="text-default-600 text-sm leading-relaxed">
          {{ surface.notice.body }}
        </p>
      </div>
    </div>

    <div
      v-if="userStore.user.id"
      class="border-default/20 bg-content1 shadow-kun-sm flex gap-3 rounded-2xl border p-4"
    >
      <KunAvatar
        :user="toKunUser(userStore.user)"
        size="md"
        :is-navigation="false"
      />
      <div class="min-w-0 flex-1">
        <CommentComposer
          :target="target"
          :seed="composerSeed"
          @submitted="(c) => emit('commentAdded', c)"
        />
      </div>
    </div>
    <div
      v-else
      class="border-default/20 bg-default-50 rounded-2xl border p-5 text-center text-sm"
    >
      请
      <button
        type="button"
        class="text-primary font-medium hover:underline"
        @click="() => startOAuthLogin()"
      >
        登录
      </button>
      后发表评论
    </div>

    <div v-if="userStore.user.id" class="flex justify-end">
      <CommentSubscribe
        :subscription="subscription"
        @set-level="(l) => emit('setLevel', l)"
      />
    </div>

    <KunLoading v-if="pending" description="加载评论中..." />

    <div v-else-if="groups.length" class="space-y-8">
      <CommentRow
        v-for="g in groups"
        :key="g.root.id"
        :comment="g.root"
        :target="target"
        :depth="0"
        :can-moderate="canModerate"
        :expanded="expandedRoots.has(g.root.id)"
        :reply-count="hiddenReplyCount(g)"
        @liked="(id, l) => emit('liked', id, l)"
        @reply-added="(r) => emit('commentAdded', r)"
        @edited="(u) => emit('edited', u)"
        @removed="(id) => emit('removed', id)"
        @toggle-expand="(id) => emit('toggleExpand', id)"
      >
        <template #replies>
          <div v-if="visibleReplies(g).length" class="mt-4 space-y-4">
            <CommentRow
              v-for="r in visibleReplies(g)"
              :key="r.id"
              :comment="r"
              :target="target"
              :depth="1"
              :can-moderate="canModerate"
              @liked="(id, l) => emit('liked', id, l)"
              @reply-added="(rr) => emit('commentAdded', rr)"
              @edited="(u) => emit('edited', u)"
              @removed="(id) => emit('removed', id)"
            />
          </div>
        </template>
      </CommentRow>
    </div>

    <KunNull v-else :description="surface.emptyDescription" />

    <!--
      Load more, not a paginator: the wall is keyset by post number and the
      upstream face answers a cursor, so there is no page count to render.
    -->
    <div v-if="hasMore" class="flex justify-center">
      <KunButton
        variant="light"
        color="primary"
        :loading="loadingMore"
        :disabled="loadingMore"
        @click="emit('loadMore')"
      >
        加载更多评论
      </KunButton>
    </div>
  </div>
</template>
