<script setup lang="ts">
import {
  SUPPORTED_RESOURCE_LINK_MAP,
  SUPPORTED_TYPE_MAP,
  SUPPORTED_LANGUAGE_MAP,
  SUPPORTED_PLATFORM_MAP
} from '~/constants/resource'
import {
  GALGAME_AGE_LIMIT_DETAIL,
  GALGAME_AGE_LIMIT_MAP
} from '~/constants/galgame'
import type { CommentTarget } from '~/shared/utils/commentTarget'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const resourceId = computed(() => Number(route.params.id))

const {
  data: detail,
  pending,
  refresh
} = await useAsyncData<PatchResourceDetail | null>(
  () => `resource-${resourceId.value}`,
  async () => {
    const res = await api.get<PatchResourceDetail>(
      `/resource/${resourceId.value}`
    )
    return res.code === 0 ? res.data : null
  },
  { deep: true }
)

const resource = computed(() => detail.value?.resource ?? null)

const noteHtml = computed(() => resource.value?.note_html ?? '')

const patchName = computed(() =>
  detail.value?.patch ? getPreferredLanguageText(detail.value.patch.name) : ''
)

const patchAlias = computed(() => {
  const p = detail.value?.patch
  if (!p) return [] as string[]
  return Object.values(p.name).filter(
    (v): v is string => !!v && v !== patchName.value
  )
})

const resourceTitle = computed(() => {
  const r = resource.value
  if (!r) return ''
  return r.name || `${patchName.value} 的补丁资源`
})

const updateTimeLabel = computed(() => {
  const r = resource.value
  if (!r) return ''
  return formatDistanceToNow((r.update_time as string) || r.created)
})

const mapJoin = (arr: string[] | undefined, m: Record<string, string>) =>
  (arr ?? []).map((k) => m[k] ?? k).join('')

const titleName = computed(() => patchName.value || resource.value?.name || '')
const titleSuffix = computed(() => {
  const r = resource.value
  if (!r) return '资源下载'
  return (
    mapJoin(r.platform, SUPPORTED_PLATFORM_MAP) +
    mapJoin(r.language, SUPPORTED_LANGUAGE_MAP) +
    (r.model_name || '') +
    mapJoin(r.type, SUPPORTED_TYPE_MAP) +
    '资源下载'
  )
})

const composedTitle = computed(() =>
  resource.value ? titleName.value + titleSuffix.value : '资源下载'
)

const storageLabel = computed(() =>
  resource.value
    ? (SUPPORTED_RESOURCE_LINK_MAP[resource.value.storage] ??
      resource.value.storage)
    : ''
)
const storageIcon = computed(() =>
  resource.value?.storage === 's3' ? 'lucide:cloud' : 'lucide:link'
)

const {
  items: revisionItems,
  total: revisionTotal,
  totalPages: revisionTotalPages,
  hasHistory,
  pending: revisionsPending,
  page: revisionPage
} = useResourceRevisions(resourceId)

const commentTarget = computed<CommentTarget>(() => ({
  kind: 'resource',
  resourceId: resourceId.value,
  galgameId: detail.value?.resource.galgame_id ?? 0
}))

const {
  items: commentItems,
  total: commentTotal,
  totalPages: commentTotalPages,
  pending: commentsPending,
  page: commentPage,
  expandedRoots,
  toggleExpand,
  onLiked,
  onCommentAdded,
  onReplyAdded,
  onEdited: onCommentEdited,
  onRemoved
} = useCommentList(commentTarget)

const PANEL_GROUP = 'resource-detail'
const activePanel = ref('download')

const panelTabs = computed(() => {
  const tabs = [
    { value: 'download', textValue: '资源下载', icon: 'lucide:download-cloud' }
  ]
  if (hasHistory.value) {
    tabs.push({
      value: 'history',
      textValue: `更改历史 ${revisionTotal.value}`,
      icon: 'lucide:history'
    })
  }
  return tabs
})

const onDownload = () => {
  if (!resource.value) return
  api.put(`/patch/resource/${resource.value.id}/download`).catch(() => {})
  if (detail.value) detail.value.resource.download += 1
}

const isResourceFavorite = computed({
  get: () => resource.value?.is_favorite ?? false,
  set: (v) => {
    if (resource.value) resource.value.is_favorite = v
  }
})
const onResourceFavoriteChange = async (active: boolean) => {
  if (!resource.value) return
  if (!requireLogin()) {
    isResourceFavorite.value = !active
    return
  }
  const res = await api.put<{ favorited: boolean }>(
    `/patch/resource/${resource.value.id}/favorite`
  )
  if (res.code === 0) {
    isResourceFavorite.value = res.data.favorited
    useKunMessage(
      res.data.favorited
        ? '已收藏此资源，下载链接或文件更新时会通知你'
        : '已取消收藏',
      'success'
    )
  } else {
    isResourceFavorite.value = !active
    useKunMessage(res.message || '操作失败', 'error')
  }
}

const onResourceEdited = (updated: PatchResourceHtml) => {
  if (detail.value) detail.value.resource = updated
}

const recName = (r: PatchResource) =>
  r.name || (r.patch ? getPreferredLanguageText(r.patch.name) : '补丁资源')

useResourceSeo(detail, { title: composedTitle, commentCount: commentTotal })
</script>

<template>
  <div class="container mx-auto my-4">
    <KunLoading v-if="pending" description="加载资源中..." />

    <template v-else-if="detail && resource">
      <div
        class="bg-content1 shadow-kun-sm mb-6 overflow-hidden rounded-3xl"
      >
        <div class="flex flex-col gap-5 p-6 sm:flex-row sm:p-8">
          <NuxtLink
            v-if="detail.patch"
            :to="`/galgame/${detail.patch.id}`"
            class="group shrink-0"
          >
            <KunImage
              :src="resolveBannerUrl(detail.patch) || '/kungalgame-trans.webp'"
              :alt="patchName"
              :aspect-ratio="resolveBannerAspectRatio(detail.patch)"
              :thumbhash="resolveBannerThumbhash(detail.patch)"
              class-name="border-default/20 bg-default-100 w-full overflow-hidden rounded-2xl border shadow-lg sm:w-72"
              image-class-name="transition-transform duration-300 group-hover:scale-[1.02]"
            />
          </NuxtLink>

          <div class="flex min-w-0 flex-1 flex-col gap-3">
            <p
              class="text-default-500 hidden text-xs tracking-[0.3em] uppercase sm:block"
            >
              Galgame 补丁资源下载
            </p>

            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-2xl font-bold break-words sm:text-3xl"><NuxtLink
                  v-if="detail.patch"
                  :to="`/galgame/${detail.patch.id}?tab=resource`"
                  class="hover:text-primary transition-colors"
                >{{ titleName }}</NuxtLink><template v-else>{{ titleName }}</template>{{ titleSuffix }}</h1>
              <KunTooltip
                v-if="detail.patch"
                :text="GALGAME_AGE_LIMIT_DETAIL[detail.patch.content_limit]"
                position="right"
              >
                <KunChip
                  :color="
                    detail.patch.content_limit === 'sfw' ? 'success' : 'danger'
                  "
                  variant="flat"
                >
                  {{ GALGAME_AGE_LIMIT_MAP[detail.patch.content_limit] }}
                </KunChip>
              </KunTooltip>
            </div>

            <div
              v-if="patchAlias.length"
              class="flex flex-wrap gap-x-3 gap-y-1"
            >
              <span
                v-for="alias in patchAlias"
                :key="alias"
                class="text-default-500 text-xs"
              >
                {{ alias }}
              </span>
            </div>

            <KunPatchAttribute
              v-if="detail.patch"
              :types="detail.patch.type"
              :languages="detail.patch.language"
              :platforms="detail.patch.platform"
              size="sm"
            />

            <div v-if="detail.patch" class="flex flex-wrap gap-3 pt-1">
              <NuxtLink :to="`/galgame/${detail.patch.id}`">
                <KunButton
                  color="primary"
                  variant="flat"
                  size="sm"
                  rounded="full"
                >
                  <KunIcon name="lucide:info" class="size-4" />
                  查看 Galgame 介绍
                </KunButton>
              </NuxtLink>
              <NuxtLink :to="`/galgame/${detail.patch.id}?tab=resource`">
                <KunButton
                  color="secondary"
                  variant="flat"
                  size="sm"
                  rounded="full"
                >
                  <KunIcon name="lucide:layers" class="size-4" />
                  查看全部资源
                </KunButton>
              </NuxtLink>
            </div>
          </div>
        </div>
      </div>

      <KunAdAIEroBanner class-name="mb-6 hidden sm:block" />

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div class="space-y-6 lg:col-span-2">
          <KunCard :bordered="true" class-name="rounded-2xl">
            <div class="space-y-4 p-2">
              <div class="flex items-start justify-between gap-3">
                <div class="space-y-1">
                  <h2 class="text-xl font-bold break-words sm:text-2xl">
                    {{ resourceTitle }}
                  </h2>
                  <p class="text-default-500 text-sm">
                    该补丁资源最后更新于 {{ updateTimeLabel }}
                  </p>
                </div>
                <ResourceDetailActions
                  :resource="resource"
                  :galgame-name="patchName"
                  @edited="onResourceEdited"
                  @refresh="refresh"
                />
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <KunChip color="secondary" variant="flat" size="sm">
                  <KunIcon :name="storageIcon" class="size-3.5" />
                  {{ storageLabel }}
                </KunChip>
                <KunChip color="warning" variant="flat" size="sm">
                  <KunIcon name="lucide:database" class="size-3.5" />
                  {{ resource.size }}
                </KunChip>
                <span
                  class="text-default-500 inline-flex items-center gap-1.5 text-sm"
                >
                  <KunIcon name="lucide:download" class="size-4" />
                  {{ formatNumber(resource.download) }} 次下载
                </span>
              </div>

              <KunPatchAttribute
                :types="resource.type"
                :languages="resource.language"
                :platforms="resource.platform"
                :model-name="resource.model_name"
                :storage="resource.storage"
                :storage-size="resource.size"
              />

              <div
                class="border-default/20 flex items-center gap-2 border-t pt-4"
              >
                <KunAvatar :user="resource.user" size="sm" />
                <div class="text-sm leading-tight">
                  <NuxtLink
                    v-if="resource.user?.id"
                    :to="`/user/${resource.user.id}/resource`"
                    class="hover:text-primary font-medium transition-colors"
                  >
                    {{ resource.user?.name ?? '已注销用户' }}
                  </NuxtLink>
                  <span v-else class="font-medium">已注销用户</span>
                  <div class="text-default-500 text-xs">
                    发布于
                    {{
                      formatDate(resource.created, {
                        isShowYear: true,
                        isPrecise: true
                      })
                    }}
                  </div>
                </div>
              </div>

              <div class="space-y-2">
                <KunReaction
                  v-model="isResourceFavorite"
                  icon="lucide:star"
                  color="warning"
                  size="md"
                  @change="onResourceFavoriteChange"
                >
                  {{ isResourceFavorite ? '已收藏资源' : '收藏资源' }}
                </KunReaction>

                <p
                  :class="
                    cn(
                      'flex items-center gap-1.5 text-xs',
                      resource.is_favorite ? 'text-warning' : 'text-default-500'
                    )
                  "
                >
                  <KunIcon name="lucide:bell" class="size-3.5 shrink-0" />
                  <span>{{
                    resource.is_favorite
                      ? '已收藏此资源，下载链接或文件更新时会通知你'
                      : '收藏此资源，下载链接或文件更新时通知你'
                  }}</span>
                </p>
              </div>

              <KunContent
                v-if="noteHtml"
                :content="noteHtml"
                class-name="border-default/20 border-t pt-4 text-sm"
              />
              <p
                v-else-if="resource.note"
                class="border-default/20 border-t pt-4 text-sm whitespace-pre-wrap"
              >
                {{ resource.note }}
              </p>
            </div>
          </KunCard>

          <KunAdAIEroBanner class-name="block sm:hidden" />

          <KunTab
            v-model="activePanel"
            :items="panelTabs"
            :name="PANEL_GROUP"
            variant="underlined"
            color="primary"
            size="md"
          />

          <KunTabPanels v-model="activePanel" :name="PANEL_GROUP">
            <KunTabPanel value="download">
              <ResourceDownload :resource="resource" @downloaded="onDownload" />
            </KunTabPanel>

            <KunTabPanel v-if="hasHistory" value="history">
              <ResourceHistory
                v-model:page="revisionPage"
                :items="revisionItems"
                :total-pages="revisionTotalPages"
                :pending="revisionsPending"
              />
            </KunTabPanel>
          </KunTabPanels>

          <section class="space-y-5">
            <KunHeader
              :name="commentTotal ? `资源评论 ${commentTotal}` : '资源评论'"
              scale="h2"
            />
            <CommentSection
              v-model:page="commentPage"
              :target="commentTarget"
              :items="commentItems"
              :total-pages="commentTotalPages"
              :expanded-roots="expandedRoots"
              :pending="commentsPending"
              :can-moderate="userStore.isModerator"
              :mention-user="resource.user"
              @comment-added="onCommentAdded"
              @liked="onLiked"
              @reply-added="onReplyAdded"
              @edited="onCommentEdited"
              @removed="onRemoved"
              @toggle-expand="toggleExpand"
            />
          </section>
        </div>

        <aside class="space-y-3 lg:sticky lg:top-20 lg:self-start">
          <NuxtLink
            v-for="r in detail.recommendations"
            :key="r.id"
            :to="`/resource/${r.id}`"
            class="border-default/20 bg-content1 shadow-kun-sm hover:bg-primary/5 block space-y-2 rounded-2xl border p-4 transition-colors"
          >
            <p class="font-semibold line-clamp-2">{{ recName(r) }}</p>
            <p v-if="r.note" class="text-default-500 line-clamp-2 text-sm">
              {{ markdownToText(r.note) }}
            </p>
            <p class="text-default-400 text-xs">
              {{ formatDistanceToNow(r.created) }} · 由
              {{ r.user?.name ?? '已注销用户' }} 发布
            </p>
            <div
              class="text-default-500 flex items-center justify-end gap-1 text-xs"
            >
              <span>{{ r.download }} 次下载</span>
              ·
              <span>{{ r.like_count }} 个点赞</span>
            </div>
          </NuxtLink>

          <KunNull
            v-if="!detail.recommendations?.length"
            description="暂无推荐资源"
          />
        </aside>
      </div>
    </template>

    <KunNull v-else description="资源不存在" />
  </div>
</template>
