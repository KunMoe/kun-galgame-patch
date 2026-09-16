<script setup lang="ts">

const route = useRoute()
const api = useApi()
const userId = computed(() => Number(route.params.id))

const PREVIEW = 5

interface ResourceList {
  items: PatchResource[]
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
      api.get<GalgameList>(
        `/user/${userId.value}/patch?page=1&limit=${PREVIEW}`
      ),
      api.get<ResourceList>(
        `/user/${userId.value}/resource?page=1&limit=${PREVIEW}`
      ),
      // Keyset, so no total: the 更多 link is shown whenever a cursor came back.
      api.get<PatchCommentFeed>(
        `/user/${userId.value}/comment?limit=${PREVIEW}`
      )
    ])
    return {
      galgames: galgames.code === 0 ? galgames.data : { items: [], total: 0 },
      resources:
        resources.code === 0 ? resources.data : { items: [], total: 0 },
      comments:
        comments.code === 0 ? comments.data : { items: [], next_cursor: '' }
    }
  },
  {
    default: () => ({
      galgames: { items: [], total: 0 },
      resources: { items: [], total: 0 },
      comments: { items: [], next_cursor: '' }
    })
  }
)

const resourcePatchName = (r: PatchResource) =>
  r.patch?.name
    ? getPreferredLanguageText(r.patch.name)
    : `补丁 #${r.galgame_id}`
const commentPatchName = (c: UserComment) =>
  c.patch?.name
    ? getPreferredLanguageText(c.patch.name)
    : `补丁 #${c.galgame_id}`

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
        <GalgameList :items="data.galgames.items" />
      </section>

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
            :to="`/galgame/${r.galgame_id}?tab=resource`"
            class="border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 block rounded-lg border p-3 transition-colors"
          >
            <div class="flex items-center justify-between gap-3">
              <span class="text-primary min-w-0 truncate text-sm">
                {{ resourcePatchName(r) }}
              </span>
              <span class="text-default-400 shrink-0 text-xs">
                {{ formatDistanceToNow(r.created) }}
              </span>
            </div>
            <p v-if="r.name" class="text-default-600 mt-1 truncate text-sm">
              {{ r.name }}
            </p>
          </NuxtLink>
        </div>
      </section>

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
            :to="c.link"
            class="border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 block rounded-lg border p-3 transition-colors"
          >
            <div class="text-default-500 mb-1 text-xs">
              {{ c.resource_id ? '评论了' : '评论在' }}
              <span class="text-primary">{{ commentPatchName(c) }}</span>
              <template v-if="c.resource_id">的补丁资源</template>
            </div>
            <p class="line-clamp-2 text-sm whitespace-pre-wrap">
              {{ c.content }}
            </p>
            <div
              class="text-default-400 mt-1.5 flex items-center gap-4 text-xs"
            >
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
