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

const route = useRoute()
const api = useApi()
const { requireLogin } = useAuthModal()

const resourceId = computed(() => Number(route.params.id))

const { data: detail, pending } = await useAsyncData<PatchResourceDetail | null>(
  () => `resource-${resourceId.value}`,
  async () => {
    const res = await api.get<PatchResourceDetail>(
      `/resource/${resourceId.value}`
    )
    return res.code === 0 ? res.data : null
  }
)

const resource = computed(() => detail.value?.resource ?? null)

// note_html is server-rendered (goldmark, no html.WithUnsafe → already
// sanitized at the source), so bind it directly without a client sanitizer.
const noteHtml = computed(() => resource.value?.note_html ?? '')

const patchName = computed(() =>
  detail.value?.patch ? getPreferredLanguageText(detail.value.patch.name) : ''
)

// Alternate-language names of the owning game (shown under the game title in
// the header, mirroring patch/[id].vue).
const patchAlias = computed(() => {
  const p = detail.value?.patch
  if (!p) return [] as string[]
  return Object.values(p.name).filter(
    (v): v is string => !!v && v !== patchName.value
  )
})

const bannerSrc = computed(() =>
  resolveBannerUrl(detail.value?.patch, 'mini')
)

// The resource's OWN title — this is what users came to see ("某某汉化补丁").
// Falls back to "<游戏名> 的补丁资源" when the uploader left it blank.
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

// Composed SEO title (drives <title> only, not the visible heading):
//   {gameName}{platforms}{languages}{modelName}{types}资源下载
// e.g. ヴァンパイアクルセイダーズWindows简体中文claude-opus-4.7AI 翻译补丁资源下载
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

const downloadLinks = computed(() =>
  resolveDownloadLinks(resource.value?.storage, resource.value?.content)
)

// status != 0 → download disabled (e.g. pulled for virus). The backend withholds
// content/code/password for disabled resources, so downloadLinks is empty here;
// show an explicit notice instead of a bare "暂无下载链接".
const isResourceDisabled = computed(() => (resource.value?.status ?? 0) !== 0)

// ─── Download (fire-and-forget counter bump) ──────────
const onDownload = () => {
  if (!resource.value) return
  api.put(`/patch/resource/${resource.value.id}/download`).catch(() => {})
  if (detail.value) detail.value.resource.download += 1
}

// ─── Favorite THIS resource (update subscription) ──
// Notifies you when this resource's download link / file changes. Game-level
// 点赞 / 收藏游戏 live on the game page (/patch/:id) — this page is scoped to the
// single resource, so it only exposes 收藏资源 (removes the like/favorite mix-up).
const resourceFavoriting = ref(false)
const toggleResourceFavorite = async () => {
  if (!resource.value) return
  if (!requireLogin()) return
  resourceFavoriting.value = true
  try {
    const res = await api.put<{ favorited: boolean }>(
      `/patch/resource/${resource.value.id}/favorite`
    )
    if (res.code === 0) {
      resource.value.is_favorite = res.data.favorited
      useKunMessage(
        res.data.favorited
          ? '已收藏此资源，下载链接或文件更新时会通知你'
          : '已取消收藏',
        'success'
      )
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    resourceFavoriting.value = false
  }
}

// ─── Recommendations preview helper ───────────────────
const recName = (r: PatchResource) =>
  r.name || (r.patch ? getPreferredLanguageText(r.patch.name) : '补丁资源')

// SEO contract (same shape as patch/[id].vue):
//   - loaded + sfw owning patch → full SEO (composed title carries
//     game+platform+language+model+type for long-tail keywords; banner as
//     og image; description from note_html stripped to plain text)
//   - loaded + nsfw owning patch → disable (resource page exposes patch
//     name + note → must not index)
//   - null / not-found → disable
const noteText = computed(() =>
  noteHtml.value ? noteHtml.value.replace(/<[^>]+>/g, '').slice(0, 140) : ''
)
if (
  detail.value &&
  resource.value &&
  detail.value.patch &&
  detail.value.patch.content_limit === 'sfw'
) {
  useKunSeoMeta({
    title: composedTitle.value,
    description: noteText.value || `${patchName.value} 的补丁资源下载`,
    ogType: 'article',
    ogImage: bannerSrc.value || undefined
  })
} else {
  useKunDisableSeo(composedTitle.value || '补丁资源')
}
</script>

<template>
  <div class="container mx-auto my-4">
    <KunLoading v-if="pending" description="加载资源中..." />

    <template v-else-if="detail && resource">
      <!-- ── Game header (basic game info only) ──────────── -->
      <div
        class="border-default/20 bg-content1/50 mb-6 overflow-hidden rounded-3xl border"
      >
        <div class="flex flex-col gap-5 p-6 sm:flex-row sm:p-8">
          <NuxtLink
            v-if="detail.patch"
            :to="`/patch/${detail.patch.id}/introduction`"
            class="group shrink-0"
          >
            <KunImage
              :src="resolveBannerUrl(detail.patch) || '/kungalgame-trans.webp'"
              :alt="patchName"
              aspect-ratio="16 / 9"
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
              <!-- Composed title (game name + platform/language/model/type +
                   资源下载), the long-tail format matching the SEO <title>. The
                   game name is a link to the patch's resource page; the
                   attribute suffix follows as plain text with no space, so it
                   reads as one title. -->
              <h1 class="text-2xl font-bold break-words sm:text-3xl"><NuxtLink
                  v-if="detail.patch"
                  :to="`/patch/${detail.patch.id}/resource`"
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
              <NuxtLink :to="`/patch/${detail.patch.id}/introduction`">
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
              <NuxtLink :to="`/patch/${detail.patch.id}/resource`">
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

      <!-- AIEro ad banner — desktop only (mobile copy sits in the main
           column, mirroring the legacy KunResourceDetail placement). -->
      <KunAdAIEroBanner class-name="mb-6 hidden sm:block" />

      <!-- ── Body grid (resource details) ─────────────────── -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <!-- main column -->
        <div class="space-y-6 lg:col-span-2">
          <KunCard :bordered="true" class-name="rounded-2xl">
            <div class="space-y-4 p-2">
              <!-- Resource title — the patch resource's own name, now visible. -->
              <div class="space-y-1">
                <h2 class="text-xl font-bold break-words sm:text-2xl">
                  {{ resourceTitle }}
                </h2>
                <p class="text-default-500 text-sm">
                  该补丁资源最后更新于 {{ updateTimeLabel }}
                </p>
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

              <!-- publisher: avatar + name, clickable → user profile -->
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

              <!-- This page is scoped to ONE resource, so it only exposes
                   收藏资源 (subscribe to this resource's updates). Game-level
                   点赞 / 收藏游戏 live on the game page (/patch/:id). -->
              <div class="space-y-2">
                <KunButton
                  :variant="resource.is_favorite ? 'flat' : 'bordered'"
                  :color="resource.is_favorite ? 'warning' : 'default'"
                  size="md"
                  rounded="full"
                  :loading="resourceFavoriting"
                  :disabled="resourceFavoriting"
                  @click="toggleResourceFavorite"
                >
                  <KunIcon
                    name="lucide:star"
                    :class="cn('size-4', resource.is_favorite && 'fill-current')"
                  />
                  {{ resource.is_favorite ? '已收藏资源' : '收藏资源' }}
                </KunButton>

                <!-- A star alone can't say "notify" — spell out 收藏资源. -->
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

              <!-- Resource note (备注) -->
              <div
                v-if="noteHtml"
                class="kun-prose border-default/20 border-t pt-4 text-sm"
                v-html="noteHtml"
              />
              <p
                v-else-if="resource.note"
                class="border-default/20 border-t pt-4 text-sm whitespace-pre-wrap"
              >
                {{ resource.note }}
              </p>
            </div>
          </KunCard>

          <!-- AIEro ad banner — mobile only (desktop copy is above the grid) -->
          <KunAdAIEroBanner class-name="block sm:hidden" />

          <!-- Download card -->
          <div
            class="border-success/40 bg-success/10 space-y-4 rounded-2xl border p-5"
          >
            <div class="flex items-center gap-2">
              <KunIcon
                name="lucide:download-cloud"
                class="text-success size-5"
              />
              <h2 class="text-lg font-semibold">资源下载</h2>
            </div>

            <div
              v-if="isResourceDisabled"
              class="border-danger/30 bg-danger/10 text-danger-700 flex items-center gap-2 rounded-xl border p-3 text-sm"
            >
              <KunIcon name="lucide:shield-alert" class="size-4 shrink-0" />
              <span>
                该资源已被禁用下载（可能存在安全风险，或应发布者 / 管理员要求下架），暂时无法获取下载链接。
              </span>
            </div>

            <p v-if="!isResourceDisabled" class="text-default-500 text-sm">
              点击下方链接下载（共 {{ downloadLinks.length }} 个）
            </p>

            <div v-if="!isResourceDisabled" class="space-y-2">
              <div
                v-for="(lnk, i) in downloadLinks"
                :key="i"
                class="border-success/40 bg-background hover:border-success focus-within:border-success flex items-center gap-3 rounded-xl border p-3 transition-colors"
              >
                <a
                  :href="lnk"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="hover:text-success group flex min-w-0 flex-1 items-center gap-3 transition-colors"
                  @click="onDownload"
                >
                  <span
                    class="bg-success/15 text-success flex size-9 shrink-0 items-center justify-center rounded-lg"
                  >
                    <KunIcon name="lucide:download" class="size-5" />
                  </span>
                  <span class="min-w-0 flex-1 truncate text-sm">{{ lnk }}</span>
                  <KunIcon
                    name="lucide:external-link"
                    class="text-default-400 group-hover:text-success size-4 shrink-0"
                  />
                </a>
                <KunButton
                  variant="light"
                  color="success"
                  size="sm"
                  is-icon-only
                  class-name="shrink-0"
                  aria-label="复制下载链接"
                  title="复制下载链接"
                  @click="useKunCopy(lnk)"
                >
                  <KunIcon name="lucide:copy" class="size-4" />
                </KunButton>
              </div>
              <p
                v-if="!downloadLinks.length"
                class="text-default-500 text-sm"
              >
                暂无可用下载链接
              </p>
            </div>

            <div
              v-if="resource.code || resource.password"
              class="flex flex-wrap gap-2"
            >
              <KunCopy
                v-if="resource.code"
                :text="resource.code"
                :name="`提取码: ${resource.code}`"
                color="secondary"
                variant="flat"
                size="sm"
              />
              <KunCopy
                v-if="resource.password"
                :text="resource.password"
                :name="`解压密码: ${resource.password}`"
                color="secondary"
                variant="flat"
                size="sm"
              />
            </div>

            <div
              v-if="resource.blake3 && resource.storage !== 'user'"
              class="border-success/30 space-y-2 border-t pt-3"
            >
              <p class="text-default-500 text-xs">
                BLAKE3 校验码，可校验下载文件完整性
              </p>
              <div class="flex flex-wrap items-center gap-2">
                <code
                  class="bg-background/60 max-w-full truncate rounded-lg px-2 py-1 text-xs"
                >
                  {{ resource.blake3 }}
                </code>
                <KunCopy
                  :text="resource.blake3"
                  name="复制"
                  size="sm"
                  variant="flat"
                />
                <NuxtLink
                  :to="`/check-hash?hash=${resource.blake3}&content=${encodeURIComponent(resource.content || '')}`"
                >
                  <KunButton
                    size="sm"
                    variant="flat"
                    color="primary"
                    rounded="full"
                  >
                    <KunIcon name="lucide:shield-check" class="size-3.5" />
                    前往校验页面
                  </KunButton>
                </NuxtLink>
              </div>
            </div>
          </div>
        </div>

        <!-- sidebar: patch resource recommendations (no wrapper / heading) -->
        <aside class="space-y-3 lg:sticky lg:top-20 lg:self-start">
          <NuxtLink
            v-for="r in detail.recommendations"
            :key="r.id"
            :to="`/resource/${r.id}`"
            class="border-default/20 bg-background hover:border-primary hover:bg-primary/5 block space-y-2 rounded-2xl border p-4 transition-colors"
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
