<script setup lang="ts">
// 动态 — the user-profile overview / landing tab (KunAvatar links here, and
// /user/:id redirects here). Shows a compact "recent activity" digest by
// reusing the same per-tab endpoints with a small limit, so there's no new
// backend surface — each section links to its full tab for more.
const route = useRoute()
const api = useApi()
const userId = computed(() => Number(route.params.id))

const PREVIEW = 5

interface CommentList {
  items: UserComment[]
  total: number
}
interface ResourceList {
  items: UserResourceItem[]
  total: number
}
interface GalgameList {
  items: GalgameCard[]
  total: number
}

const { data, pending } = await useAsyncData(
  () => `user-${userId.value}-activity`,
  async () => {
    const [galgames, resources, comments] = await Promise.all([
      api.get<GalgameList>(`/user/${userId.value}/patch?page=1&limit=${PREVIEW}`),
      api.get<ResourceList>(`/user/${userId.value}/resource?page=1&limit=${PREVIEW}`),
      api.get<CommentList>(`/user/${userId.value}/comment?page=1&limit=${PREVIEW}`)
    ])
    return {
      galgames: galgames.code === 0 ? galgames.data : { items: [], total: 0 },
      resources: resources.code === 0 ? resources.data : { items: [], total: 0 },
      comments: comments.code === 0 ? comments.data : { items: [], total: 0 }
    }
  },
  {
    default: () => ({
      galgames: { items: [], total: 0 },
      resources: { items: [], total: 0 },
      comments: { items: [], total: 0 }
    })
  }
)

const resourcePatchName = (r: UserResourceItem) =>
  r.patch?.name ? getPreferredLanguageText(r.patch.name) : `补丁 #${r.galgame_id}`
const commentPatchName = (c: UserComment) =>
  c.patch?.name ? getPreferredLanguageText(c.patch.name) : `补丁 #${c.galgame_id}`

const isEmpty = computed(
  () =>
    !data.value?.galgames.items.length &&
    !data.value?.resources.items.length &&
    !data.value?.comments.items.length
)
</script>

<template>
  <div>
    <KunLoading v-if="pending" description="加载中..." />
    <KunNull v-else-if="isEmpty" description="该用户暂无动态" />
    <div v-else class="space-y-8">
      <!-- 最近发布的 Galgame -->
      <section v-if="data?.galgames.items.length" class="space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="flex items-center gap-2 text-lg font-semibold">
            <KunIcon name="lucide:gamepad-2" class="text-primary size-5" />
            最近发布的 Galgame
          </h2>
          <NuxtLink
            :to="`/user/${userId}/galgame`"
            class="text-default-500 hover:text-primary flex items-center gap-1 text-sm transition-colors"
          >
            查看全部
            <KunIcon name="lucide:chevron-right" class="size-4" />
          </NuxtLink>
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          <GalgameCard
            v-for="patch in data.galgames.items"
            :key="patch.id"
            :patch="patch"
          />
        </div>
      </section>

      <!-- 最近发布的补丁资源 -->
      <section v-if="data?.resources.items.length" class="space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="flex items-center gap-2 text-lg font-semibold">
            <KunIcon name="lucide:puzzle" class="text-primary size-5" />
            最近发布的补丁资源
          </h2>
          <NuxtLink
            :to="`/user/${userId}/resource`"
            class="text-default-500 hover:text-primary flex items-center gap-1 text-sm transition-colors"
          >
            查看全部
            <KunIcon name="lucide:chevron-right" class="size-4" />
          </NuxtLink>
        </div>
        <div class="space-y-2">
          <NuxtLink
            v-for="r in data.resources.items"
            :key="r.id"
            :to="`/patch/${r.galgame_id}/resource`"
            class="border-default/20 bg-background hover:bg-default-100 block rounded-lg border p-3 transition-colors"
          >
            <div class="flex items-center justify-between gap-3">
              <span class="text-primary min-w-0 truncate text-sm">
                {{ resourcePatchName(r) }}
              </span>
              <span class="text-default-400 shrink-0 text-xs">
                {{ formatDistanceToNow(r.created) }}
              </span>
            </div>
            <p
              v-if="r.name"
              class="text-default-600 mt-1 truncate text-sm"
            >
              {{ r.name }}
            </p>
          </NuxtLink>
        </div>
      </section>

      <!-- 最近的评论 -->
      <section v-if="data?.comments.items.length" class="space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="flex items-center gap-2 text-lg font-semibold">
            <KunIcon name="lucide:message-square" class="text-primary size-5" />
            最近的评论
          </h2>
          <NuxtLink
            :to="`/user/${userId}/comment`"
            class="text-default-500 hover:text-primary flex items-center gap-1 text-sm transition-colors"
          >
            查看全部
            <KunIcon name="lucide:chevron-right" class="size-4" />
          </NuxtLink>
        </div>
        <div class="space-y-2">
          <NuxtLink
            v-for="c in data.comments.items"
            :key="c.id"
            :to="`/patch/${c.galgame_id}/comment`"
            class="border-default/20 bg-background hover:bg-default-100 block rounded-lg border p-3 transition-colors"
          >
            <div class="text-default-500 mb-1 text-xs">
              评论在 <span class="text-primary">{{ commentPatchName(c) }}</span>
            </div>
            <p class="line-clamp-2 text-sm whitespace-pre-wrap">{{ c.content }}</p>
            <div class="text-default-400 mt-1.5 flex items-center gap-4 text-xs">
              <span class="flex items-center gap-1">
                <KunIcon name="lucide:thumbs-up" class="size-3.5" />
                {{ c.like_count }}
              </span>
              <span>{{ formatDistanceToNow(c.created) }}</span>
            </div>
          </NuxtLink>
        </div>
      </section>
    </div>
  </div>
</template>
