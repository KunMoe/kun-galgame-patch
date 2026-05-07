<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

const route = useRoute()
const api = useApi()

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
            <div class="flex items-center gap-1">
              <KunIcon name="lucide:heart" class="size-4" />
              {{ r.like_count }}
            </div>
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
