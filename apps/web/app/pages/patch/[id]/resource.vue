<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

const galgameId = computed(() => Number(route.params.id))

const sanitize = (html: string) =>
  DOMPurify.sanitize(html, { ADD_ATTR: ['data-uid'] })

const { data: resources, pending } = await useAsyncData<PatchResource[]>(
  () => `patch-resource-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchResource[]>(
      `/patch/${galgameId.value}/resource`
    )
    return res.code === 0 ? res.data : []
  },
  { default: () => [] }
)

// Optimistic resource-like toggle, mirroring the comment pattern: backend
// returns { liked }, we fold it onto the local row.
const toggleLike = async (r: PatchResource) => {
  if (!userStore.user.uid) {
    useKunMessage('请先登录后再点赞', 'warn')
    return
  }
  const res = await api.put<{ liked: boolean }>(
    `/patch/resource/${r.id}/like`
  )
  if (res.code === 0) {
    const liked = res.data.liked
    const prev = r.is_liked ?? false
    const delta = liked === prev ? 0 : liked ? 1 : -1
    r.is_liked = liked
    r.like_count = Math.max(0, r.like_count + delta)
  } else {
    useKunMessage(res.message || '操作失败', 'error')
  }
}
</script>

<template>
  <div class="space-y-4">
    <KunLoading v-if="pending" description="正在获取补丁资源..." />
    <div v-else-if="resources && resources.length" class="space-y-3">
      <div
        v-for="r in resources"
        :key="r.id"
        class="border-default/20 space-y-3 rounded-lg border p-4"
      >
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div>
            <h3 class="text-lg font-semibold line-clamp-2">
              {{ r.name || '补丁资源' }}
            </h3>
            <div class="text-default-500 text-xs">
              由 {{ r.user.name }} 发布于
              {{ formatDate(r.created, { isShowYear: true, isPrecise: true }) }}
            </div>
          </div>
          <KunBadge size="sm" variant="flat">{{ r.size }}</KunBadge>
        </div>

        <KunPatchAttribute
          :types="r.type"
          :languages="r.language"
          :platforms="r.platform"
          :model-name="r.model_name"
          :storage="r.storage"
          size="sm"
        />

        <div
          v-if="r.note_html"
          class="kun-prose text-default-500 text-sm"
          v-html="sanitize(r.note_html)"
        />

        <div
          v-if="r.code || r.password"
          class="text-default-500 flex flex-wrap gap-4 text-sm"
        >
          <span v-if="r.code">提取码: {{ r.code }}</span>
          <span v-if="r.password">解压密码: {{ r.password }}</span>
        </div>

        <div v-if="r.blake3" class="text-default-400 break-all text-xs">
          Hash: {{ r.blake3 }}
          <NuxtLink
            :to="`/check-hash?hash=${r.blake3}&content=${encodeURIComponent(r.content || '')}`"
            class="text-primary ml-2 hover:underline"
          >
            校验文件
          </NuxtLink>
        </div>

        <div
          class="text-default-500 flex flex-wrap items-center justify-between gap-2 pt-2 text-sm"
        >
          <div class="flex items-center gap-4">
            <button
              type="button"
              :class="
                cn(
                  'flex items-center gap-1 transition-colors',
                  r.is_liked
                    ? 'text-danger-500'
                    : 'text-default-500 hover:text-danger-500'
                )
              "
              :aria-label="r.is_liked ? '取消点赞' : '点赞'"
              @click="toggleLike(r)"
            >
              <KunIcon
                name="lucide:heart"
                :class="cn('size-4', r.is_liked ? 'fill-current' : '')"
              />
              {{ r.like_count }}
            </button>
            <div class="flex items-center gap-1">
              <KunIcon name="lucide:download" class="size-4" />
              {{ r.download }}
            </div>
          </div>
          <NuxtLink :to="`/resource/${r.id}`">
            <KunButton color="primary" size="sm">
              <KunIcon name="lucide:download" class="size-4" />
              下载
            </KunButton>
          </NuxtLink>
        </div>
      </div>
    </div>
    <KunNull v-else description="该 Galgame 暂无补丁资源" />
  </div>
</template>
