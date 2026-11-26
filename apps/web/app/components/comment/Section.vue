<script setup lang="ts">
import {
  REPLY_PREVIEW,
  type CommentGroup,
  type CommentSort,
  type CommentWall
} from '~/composables/useCommentList'
import type { CommentTarget } from '~/shared/utils/commentTarget'
import { commentSurface } from '~/shared/utils/commentTarget'

const props = withDefaults(
  defineProps<{
    target: CommentTarget
    wall: CommentWall
    mentionUser?: KunUser | null
  }>(),
  { mentionUser: null }
)

const {
  sort,
  pinnedGroups,
  groups,
  subscription,
  setLevel,
  hasComments,
  hasMore,
  loadMore,
  loadingMore,
  loadReplies,
  loadingReplies,
  pending,
  expandedRoots,
  toggleExpand,
  onLiked,
  onCommentAdded,
  onEdited,
  onRemoved,
  onRestored,
  onPinned
} = props.wall

const userStore = useUserStore()
const surface = commentSurface(props.target)

const SORT_TABS: { value: CommentSort; textValue: string }[] = [
  { value: 'oldest', textValue: '最早' },
  { value: 'newest', textValue: '最新' },
  { value: 'top', textValue: '最热' }
]

const visibleReplies = (group: CommentGroup) =>
  expandedRoots.value.has(group.root.id)
    ? group.replies
    : group.replies.slice(0, REPLY_PREVIEW)

const hiddenReplyCount = (group: CommentGroup) =>
  group.replies.length - visibleReplies(group).length

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
          @submitted="onCommentAdded"
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

    <div class="flex flex-wrap items-center justify-between gap-2">
      <KunTab
        v-model="sort"
        :items="SORT_TABS"
        variant="light"
        size="sm"
        name="comment-sort"
      />
      <CommentSubscribe
        v-if="userStore.user.id"
        :subscription="subscription"
        @set-level="setLevel"
      />
    </div>

    <KunLoading v-if="pending" description="加载评论中..." />

    <div v-else-if="hasComments" class="space-y-8">
      <CommentRow
        v-for="g in [...pinnedGroups, ...groups]"
        :key="g.root.id"
        :comment="g.root"
        :target="target"
        :depth="0"
        :expanded="expandedRoots.has(g.root.id)"
        :hidden-reply-count="hiddenReplyCount(g)"
        :remaining-reply-count="g.remaining"
        :collapsible="g.replies.length > REPLY_PREVIEW"
        :loading-replies="loadingReplies.has(g.root.id)"
        @liked="onLiked"
        @reply-added="onCommentAdded"
        @edited="onEdited"
        @removed="onRemoved"
        @restored="onRestored"
        @pinned="onPinned"
        @toggle-expand="toggleExpand"
        @load-replies="loadReplies"
      >
        <template #replies>
          <div v-if="visibleReplies(g).length" class="mt-4 space-y-4">
            <CommentRow
              v-for="r in visibleReplies(g)"
              :key="r.id"
              :comment="r"
              :target="target"
              :depth="1"
              @liked="onLiked"
              @reply-added="onCommentAdded"
              @edited="onEdited"
              @removed="onRemoved"
              @restored="onRestored"
            />
          </div>
        </template>
      </CommentRow>
    </div>

    <KunNull v-else :description="surface.emptyDescription" />

    <!--
      Load more, not a paginator: the wall is keyset and the upstream face
      answers a cursor, so there is no page count to render.
    -->
    <div v-if="hasMore && !pending" class="flex justify-center">
      <KunButton
        variant="light"
        color="primary"
        :loading="loadingMore"
        :disabled="loadingMore"
        @click="loadMore"
      >
        加载更多评论
      </KunButton>
    </div>
  </div>
</template>
