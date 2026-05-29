<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'
import {
  SUPPORTED_RESOURCE_LINK_MAP,
  SUPPORTED_TYPE_MAP,
  SUPPORTED_LANGUAGE_MAP,
  SUPPORTED_PLATFORM_MAP
} from '~/constants/resource'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

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

const noteHtml = computed(() =>
  resource.value?.note_html
    ? DOMPurify.sanitize(resource.value.note_html, { ADD_ATTR: ['data-id'] })
    : ''
)

const patchName = computed(() =>
  detail.value?.patch ? getPreferredLanguageText(detail.value.patch.name) : ''
)

const bannerSrc = computed(() =>
  resolveBannerUrl(detail.value?.patch, 'mini')
)

// Composed title:
//   {gameName}{platforms}{languages}{modelName}{types}资源下载
// e.g. ヴァンパイアクルセイダーズWindows简体中文claude-opus-4.7AI 翻译补丁资源下载
//
// Split into the leading game name + the attribute suffix so the H1 can render
// the name as a link to the owning patch's resource page, while composedTitle
// (name + suffix, a plain string) still drives the SEO <title>.
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
  (resource.value?.content ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
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

// ─── Like (resource) ──────────────────────────────────
const liking = ref(false)
const toggleLike = async () => {
  if (!resource.value) return
  if (!userStore.user.id) {
    useKunMessage('请先登录后再点赞', 'warn')
    return
  }
  liking.value = true
  try {
    const res = await api.put<{ liked: boolean }>(
      `/patch/resource/${resource.value.id}/like`
    )
    if (res.code === 0) {
      const liked = res.data.liked
      const prev = resource.value.is_liked ?? false
      resource.value.is_liked = liked
      resource.value.like_count = Math.max(
        0,
        resource.value.like_count + (liked === prev ? 0 : liked ? 1 : -1)
      )
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    liking.value = false
  }
}

// ─── Favorite (the owning galgame/patch) ──────────────
const favorited = ref(false)
watch(
  detail,
  (d) => {
    favorited.value = !!d?.patch_is_favorite
  },
  { immediate: true }
)
const favoriting = ref(false)
const toggleFavorite = async () => {
  if (!detail.value?.patch) return
  if (!userStore.user.id) {
    useKunMessage('请先登录后再收藏', 'warn')
    return
  }
  favoriting.value = true
  try {
    const res = await api.put<{ favorited: boolean }>(
      `/patch/${detail.value.patch.id}/favorite`
    )
    if (res.code === 0) favorited.value = res.data.favorited
    else useKunMessage(res.message || '操作失败', 'error')
  } finally {
    favoriting.value = false
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
//
// detail.patch.content_limit is wiki-sourced via the resource detail enricher
// (see common/handler.go GetResourceDetail → enricher.EnrichPatch which calls
// applyGalgame). content_limit was D12-moved off the local patch row, but
// the enricher restamps it on GalgameCard so this field IS current.
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
      <!-- ── Hero ─────────────────────────────────────── -->
      <div
        class="border-default/20 relative mb-6 overflow-hidden rounded-3xl border"
      >
        <div class="absolute inset-0">
          <KunImage
            v-if="bannerSrc"
            :src="bannerSrc"
            :alt="patchName"
            class-name="block size-full"
            image-class-name="scale-110 blur-2xl"
          />
          <div
            class="from-background via-background/85 to-background/60 absolute inset-0 bg-gradient-to-t"
          />
        </div>

        <div class="relative flex flex-col gap-4 p-6 sm:flex-row sm:p-8">
          <NuxtLink
            v-if="detail.patch"
            :to="`/patch/${detail.patch.id}/introduction`"
            class="group shrink-0"
          >
            <KunImage
              v-if="bannerSrc"
              :src="bannerSrc"
              :alt="patchName"
              aspect-ratio="16 / 9"
              class-name="border-default/20 bg-default-100 w-full rounded-2xl border shadow-lg sm:w-64"
              image-class-name="transition-transform duration-300 group-hover:scale-[1.02]"
            />
          </NuxtLink>

          <div class="flex min-w-0 flex-1 flex-col justify-end gap-3">
            <NuxtLink
              v-if="detail.patch"
              :to="`/patch/${detail.patch.id}/introduction`"
              class="text-default-500 hover:text-primary inline-flex w-fit items-center gap-1 text-sm transition-colors"
            >
              <KunIcon name="lucide:corner-up-left" class="size-4" />
              {{ patchName }}
            </NuxtLink>

            <!-- Game name is a link to the owning patch's resource page; the
                 attribute suffix follows as plain text (no space between, to
                 keep the same composed look as the SEO title). -->
            <h1
              class="text-2xl font-bold break-words sm:text-3xl lg:text-[2rem] lg:leading-tight"
            ><NuxtLink
                v-if="detail.patch"
                :to="`/patch/${detail.patch.id}/resource`"
                class="hover:text-primary transition-colors hover:underline"
              >{{ titleName }}</NuxtLink><template v-else>{{ titleName }}</template>{{ titleSuffix }}</h1>

            <div class="flex flex-wrap items-center gap-2">
              <KunChip color="secondary" variant="flat" size="sm">
                <KunIcon :name="storageIcon" class="size-3.5" />
                {{ storageLabel }}
              </KunChip>
              <KunChip color="warning" variant="flat" size="sm">
                <KunIcon name="lucide:database" class="size-3.5" />
                {{ resource.size }}
              </KunChip>
            </div>

            <!-- publisher: avatar + name, clickable → user profile -->
            <div class="flex items-center gap-2">
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

            <!-- like + favorite, integrated into the hero -->
            <div class="mt-1 flex flex-wrap items-center gap-2">
              <KunButton
                :variant="resource.is_liked ? 'bordered' : 'bordered'"
                :color="resource.is_liked ? 'danger' : 'default'"
                size="sm"
                rounded="full"
                :loading="liking"
                :disabled="liking"
                @click="toggleLike"
              >
                <KunIcon
                  name="lucide:heart"
                  :class="cn('size-4', resource.is_liked && 'fill-current')"
                />
                点赞
                <span class="text-default-400">{{ resource.like_count }}</span>
              </KunButton>

              <KunButton
                v-if="detail.patch"
                variant="bordered"
                :color="favorited ? 'warning' : 'default'"
                size="sm"
                rounded="full"
                :loading="favoriting"
                :disabled="favoriting"
                @click="toggleFavorite"
              >
                <KunIcon
                  name="lucide:star"
                  :class="cn('size-4', favorited && 'fill-current')"
                />
                {{ favorited ? '已收藏' : '收藏游戏' }}
              </KunButton>

              <span
                class="text-default-500 inline-flex items-center gap-1 text-sm"
              >
                <KunIcon name="lucide:download" class="size-4" />
                {{ formatNumber(resource.download) }} 次下载
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- ── Body grid ────────────────────────────────── -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <!-- main column -->
        <div class="space-y-6 lg:col-span-2">
          <KunCard :bordered="true" class-name="rounded-2xl">
            <div class="space-y-4 p-2">
              <KunPatchAttribute
                :types="resource.type"
                :languages="resource.language"
                :platforms="resource.platform"
                :model-name="resource.model_name"
                :storage="resource.storage"
                :storage-size="resource.size"
              />

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
