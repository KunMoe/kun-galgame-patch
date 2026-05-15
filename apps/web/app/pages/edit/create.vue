<script setup lang="ts">
// Publish-galgame wizard.
//
// Implements the 4 scenarios laid out in docs/galgame_wiki/00-handbook-for-downstream.md §4:
//   A. Hit a published galgame (status=0) → go straight to patch creation.
//   B. Hit a VNDB draft (status=2)        → claim → patch creation.
//   C. Hit own pending/declined (3/4)     → jump to /me/submissions.
//   D. No match                            → submit form → status=3 awaits review.
//
// All search calls hit our backend's /galgame/search/publish proxy which adds
// the Bearer token + include_pending=true so the wiki surfaces both public
// items and the caller's own pending entries.

useKunSeoMeta({
  title: '发布 Galgame',
  description: '搜索现有条目或提交新的 Galgame 元数据'
})

const api = useApi()
const userStore = useUserStore()
const route = useRoute()

if (!userStore.isLoggedIn) {
  await navigateTo({ path: '/login', query: { from: route.fullPath } })
}

type WizardMode = 'search' | 'submit'
const mode = ref<WizardMode>('search')

// ─── Search state ─────────────────────────────────────
const searchQuery = ref('')
const searching = ref(false)
const searched = ref(false) // true after first search; gates the "no results" CTA
interface GalgameHit {
  id: number
  status: number
  vndb_id: string
  name_en_us: string
  name_ja_jp: string
  name_zh_cn: string
  name_zh_tw: string
  banner: string
  banner_image_hash?: string
}
interface SearchResult {
  items: GalgameHit[]
  pending: GalgameHit[]
  total: number
}
const results = ref<SearchResult>({ items: [], pending: [], total: 0 })

const displayName = (h: GalgameHit): string =>
  h.name_zh_cn || h.name_zh_tw || h.name_ja_jp || h.name_en_us || `#${h.id}`

const doSearch = async () => {
  const q = searchQuery.value.trim()
  if (!q) {
    useKunMessage('请输入搜索关键词', 'warn')
    return
  }
  searching.value = true
  searched.value = false
  try {
    const res = await api.get<SearchResult>(
      `/galgame/search/publish?q=${encodeURIComponent(q)}&limit=12`
    )
    if (res.code === 0) {
      results.value = res.data ?? { items: [], pending: [], total: 0 }
      searched.value = true
    } else {
      useKunMessage(res.message || '搜索失败', 'error')
    }
  } finally {
    searching.value = false
  }
}

// ─── Scenario A: pick a published galgame (status=0) ──
const selectingFor = ref<number | null>(null)
const selectPublished = async (hit: GalgameHit) => {
  selectingFor.value = hit.id
  try {
    const res = await api.post<{ id: number }>('/patch', {
      vndb_id: hit.vndb_id
    })
    if (res.code === 0 && res.data?.id) {
      useKunMessage('已关联，正在跳转到游戏页面', 'success')
      await navigateTo(`/patch/${res.data.id}/introduction`)
      return
    }
    if (res.code === 44001) {
      // Shouldn't happen because we searched and got the hit, but guard anyway.
      useKunMessage(
        'Wiki 中未找到此游戏，请刷新后重试',
        'warn'
      )
      return
    }
    useKunMessage(res.message || '关联失败', 'error')
  } finally {
    selectingFor.value = null
  }
}

// ─── Scenario B: claim a VNDB draft (status=2) ────────
const claimingFor = ref<number | null>(null)
const claimAndPublish = async (hit: GalgameHit) => {
  claimingFor.value = hit.id
  try {
    const claimRes = await api.post<{ id: number }>(
      `/galgame/${hit.id}/claim`,
      {}
    )
    if (claimRes.code !== 0) {
      useKunMessage(claimRes.message || '认领失败', 'error')
      return
    }
    // claim succeeded; now create the local patch row so the user can add resources.
    const patchRes = await api.post<{ id: number }>('/patch', {
      vndb_id: hit.vndb_id
    })
    if (patchRes.code === 0 && patchRes.data?.id) {
      useKunMessage('认领成功，+3 萌萌点已到账', 'success')
      await navigateTo(`/patch/${patchRes.data.id}/introduction`)
      return
    }
    useKunMessage(patchRes.message || '本站登记失败', 'error')
  } finally {
    claimingFor.value = null
  }
}

// ─── Scenario C: jump to /me/submissions ──────────────
const goToMine = async () => {
  await navigateTo('/me/submissions')
}

// ─── Scenario D: submit a new galgame ─────────────────

interface SubmitForm {
  vndb_id: string
  name_zh_cn: string
  name_zh_tw: string
  name_ja_jp: string
  name_en_us: string
  intro_zh_cn: string
  aliases: string
  content_limit: 'sfw' | 'nsfw'
  age_limit: 'all' | 'r18'
  original_language: string
}
const submitForm = reactive<SubmitForm>({
  vndb_id: '',
  name_zh_cn: '',
  name_zh_tw: '',
  name_ja_jp: '',
  name_en_us: '',
  intro_zh_cn: '',
  aliases: '',
  content_limit: 'sfw',
  age_limit: 'r18',
  original_language: 'ja-jp'
})

// banner file is held separately from the reactive form.
const bannerFile = ref<File | null>(null)
const bannerPreview = ref<string | null>(null)
const onBannerChange = (e: Event) => {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0] ?? null
  bannerFile.value = f
  if (bannerPreview.value) URL.revokeObjectURL(bannerPreview.value)
  bannerPreview.value = f ? URL.createObjectURL(f) : null
}
onBeforeUnmount(() => {
  if (bannerPreview.value) URL.revokeObjectURL(bannerPreview.value)
})

const startSubmit = () => {
  // Pre-fill name from the search query so users don't retype.
  if (!submitForm.name_zh_cn && !submitForm.name_ja_jp) {
    const q = searchQuery.value.trim()
    if (/[一-龥]/.test(q)) {
      submitForm.name_zh_cn = q
    } else if (/[぀-ゟ゠-ヿ]/.test(q)) {
      submitForm.name_ja_jp = q
    } else if (q) {
      submitForm.name_en_us = q
    }
  }
  mode.value = 'submit'
}

const submitting = ref(false)
const submitError = ref<string | null>(null)

const VNDBRegex = /^v\d+$/

const handleSubmit = async () => {
  submitError.value = null
  const hasName =
    submitForm.name_zh_cn.trim() ||
    submitForm.name_zh_tw.trim() ||
    submitForm.name_ja_jp.trim() ||
    submitForm.name_en_us.trim()
  if (!hasName) {
    submitError.value = '至少填写一个语言的名称'
    return
  }
  const vndb = submitForm.vndb_id.trim().toLowerCase()
  if (vndb && !VNDBRegex.test(vndb)) {
    submitError.value = 'VNDB ID 格式应为 v + 数字（留空表示无 VNDB 编号）'
    return
  }

  // Only include non-empty fields in the payload so omitempty on the wiki side
  // can keep defaults for what we didn't set.
  const payload: Record<string, unknown> = {
    content_limit: submitForm.content_limit,
    age_limit: submitForm.age_limit
  }
  if (vndb) payload.vndb_id = vndb
  if (submitForm.name_zh_cn.trim()) payload.name_zh_cn = submitForm.name_zh_cn.trim()
  if (submitForm.name_zh_tw.trim()) payload.name_zh_tw = submitForm.name_zh_tw.trim()
  if (submitForm.name_ja_jp.trim()) payload.name_ja_jp = submitForm.name_ja_jp.trim()
  if (submitForm.name_en_us.trim()) payload.name_en_us = submitForm.name_en_us.trim()
  if (submitForm.intro_zh_cn.trim()) payload.intro_zh_cn = submitForm.intro_zh_cn.trim()
  if (submitForm.aliases.trim()) payload.aliases = submitForm.aliases.trim()
  if (submitForm.original_language) payload.original_language = submitForm.original_language

  submitting.value = true
  try {
    // multipart when a banner file is attached; JSON otherwise. The backend
    // proxy handles both content types.
    let res: { code: number; message: string; data: { id?: number } | null }
    if (bannerFile.value) {
      const fd = new FormData()
      fd.append('data', JSON.stringify(payload))
      fd.append('file', bannerFile.value, bannerFile.value.name)
      const config = useRuntimeConfig()
      const base = config.public.apiBase || ''
      const raw = await $fetch
        .raw<typeof res>(`${base}/galgame/submit`, {
          method: 'POST',
          body: fd,
          credentials: 'include'
        })
        .catch((e) => e?.response)
      res = (raw?._data ?? { code: -1, message: '上传失败', data: null }) as typeof res
    } else {
      res = await api.post(`/galgame/submit`, payload)
    }

    if (res.code === 0) {
      useKunMessage(
        '提交成功！您的作品已进入审核队列，审核通过后将获得 +3 萌萌点',
        'success'
      )
      await navigateTo('/me/submissions')
      return
    }
    // Wiki business errors per docs/galgame_wiki/99-appendix.md §20xxx:
    //   20003 vndb_id 格式错  20004 vndb_id 已存在  20009 配额已用尽
    if (res.code === 20009) {
      submitError.value = '今日投稿配额已用尽（默认 5 条），明日再来。'
    } else if (res.code === 20004) {
      submitError.value =
        '该 VNDB ID 已被收录，请回到搜索步骤选择已存在的条目。'
    } else if (res.code === 20003) {
      submitError.value = 'VNDB ID 格式无效（应为 v + 数字）'
    } else {
      submitError.value = res.message || '提交失败'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="container mx-auto my-4 max-w-3xl px-4">
    <KunHeader
      name="发布 Galgame"
      description="先搜索 Wiki，看看是否已有条目；若没有再提交新的元数据"
    />

    <!-- ============ Mode: search ============ -->
    <div v-if="mode === 'search'" class="mt-6 space-y-4">
      <KunCard :bordered="true">
        <div class="space-y-3 p-4">
          <h2 class="text-lg font-semibold">1. 搜索 Wiki</h2>
          <p class="text-default-500 text-sm">
            支持名字（中/英/日）或 VNDB ID 搜索。
          </p>
          <form class="flex gap-2" @submit.prevent="doSearch">
            <KunInput
              v-model="searchQuery"
              placeholder="例如：Fate / フェイト / v17"
              class-name="flex-1"
            />
            <KunButton
              type="submit"
              color="primary"
              :loading="searching"
              :disabled="searching || !searchQuery.trim()"
            >
              搜索
            </KunButton>
          </form>
        </div>
      </KunCard>

      <!-- Pending (caller's own status=3/4) — always at top with high salience -->
      <KunCard
        v-if="results.pending.length > 0"
        :bordered="true"
        class-name="border-warning/40"
      >
        <div class="space-y-3 p-4">
          <div class="flex items-center gap-2">
            <KunIcon name="lucide:clock" class="text-warning size-5" />
            <h3 class="text-lg font-semibold">您已提交过的作品（等待审核）</h3>
          </div>
          <p class="text-default-500 text-sm">
            点击「查看进度」前往「我的提交」页查看审核状态、被拒原因或重新编辑。
          </p>
          <div class="space-y-2">
            <div
              v-for="hit in results.pending"
              :key="hit.id"
              class="border-default/20 flex items-center gap-3 rounded-lg border p-3"
            >
              <div class="flex-1">
                <p class="font-semibold">{{ displayName(hit) }}</p>
                <p class="text-default-500 text-xs">
                  {{ hit.vndb_id || '无 VNDB ID' }} ·
                  {{ hit.status === 3 ? '审核中' : '已拒绝（可重新提交）' }}
                </p>
              </div>
              <KunButton variant="bordered" size="sm" @click="goToMine">
                查看进度
              </KunButton>
            </div>
          </div>
        </div>
      </KunCard>

      <!-- Public hits (status=0 or status=2) -->
      <KunCard v-if="results.items.length > 0" :bordered="true">
        <div class="space-y-3 p-4">
          <h3 class="text-lg font-semibold">搜索结果</h3>
          <div class="space-y-2">
            <div
              v-for="hit in results.items"
              :key="hit.id"
              class="border-default/20 flex items-center gap-3 rounded-lg border p-3"
            >
              <div class="flex-1">
                <p class="font-semibold">{{ displayName(hit) }}</p>
                <p class="text-default-500 text-xs">
                  {{ hit.vndb_id || '无 VNDB ID' }}
                  <span v-if="hit.status === 2" class="text-warning ml-2">
                    · VNDB 草稿（认领后即发布）
                  </span>
                </p>
              </div>
              <KunButton
                v-if="hit.status === 2"
                color="warning"
                size="sm"
                :loading="claimingFor === hit.id"
                :disabled="claimingFor !== null"
                @click="claimAndPublish(hit)"
              >
                认领并发布
              </KunButton>
              <KunButton
                v-else
                color="primary"
                size="sm"
                :loading="selectingFor === hit.id"
                :disabled="selectingFor !== null"
                @click="selectPublished(hit)"
              >
                选择此条目
              </KunButton>
            </div>
          </div>
        </div>
      </KunCard>

      <!-- Empty-result CTA — "submit new" -->
      <KunCard
        v-if="searched && results.items.length === 0 && results.pending.length === 0"
        :bordered="true"
      >
        <div class="space-y-3 p-4 text-center">
          <p class="text-default-500">没有找到匹配的条目</p>
          <KunButton color="primary" @click="startSubmit">
            提交新作到 Wiki
          </KunButton>
        </div>
      </KunCard>

      <!-- Always-on CTA so users can submit even when hits exist (e.g. wrong match) -->
      <div v-if="searched" class="text-center">
        <KunButton variant="light" color="primary" @click="startSubmit">
          以上都不是？提交新作
        </KunButton>
      </div>
    </div>

    <!-- ============ Mode: submit ============ -->
    <KunCard v-else :bordered="true" class-name="mt-6">
      <div class="space-y-4 p-4">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold">2. 提交新作到 Wiki</h2>
          <KunButton variant="light" size="sm" @click="mode = 'search'">
            ← 回到搜索
          </KunButton>
        </div>

        <div class="border-default/20 bg-default-50 rounded-lg border p-3 text-sm">
          <p class="text-default-700">
            提交后状态为「待审核」，进入 admin 审核队列。审核通过后您将获得
            <strong>+3 萌萌点</strong>，并可在本站发布该游戏的补丁。
            <br />
            每天最多提交 <strong>5 条</strong>。
          </p>
        </div>

        <section class="space-y-3">
          <h3 class="font-semibold">名称（至少填一个语言）</h3>
          <label class="block">
            <span class="text-default-700 text-sm">简体中文</span>
            <KunInput v-model="submitForm.name_zh_cn" placeholder="例如：你和她和她的恋爱" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">繁體中文</span>
            <KunInput v-model="submitForm.name_zh_tw" placeholder="繁體中文名稱" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">日本語</span>
            <KunInput v-model="submitForm.name_ja_jp" placeholder="日本語タイトル" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">English</span>
            <KunInput v-model="submitForm.name_en_us" placeholder="English title" />
          </label>
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">VNDB ID（可选）</h3>
          <KunInput v-model="submitForm.vndb_id" placeholder="例如 v17（无 VNDB 编号留空）" />
          <p class="text-default-500 text-xs">
            原创 / 独立作品没有 VNDB 编号时可留空。
          </p>
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">简介（简体中文）</h3>
          <textarea
            v-model="submitForm.intro_zh_cn"
            rows="5"
            class="border-default/20 bg-background w-full rounded-lg border p-3 text-sm"
            placeholder="支持 Markdown，可选"
          />
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">Banner（可选）</h3>
          <p class="text-default-500 text-xs">
            JPEG / PNG / WebP，最大 10 MB。上传后由后端转交 image_service。
          </p>
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            class="border-default/20 bg-background w-full rounded-lg border p-2 text-sm"
            @change="onBannerChange"
          />
          <div v-if="bannerPreview" class="mt-2">
            <img
              :src="bannerPreview"
              alt="banner 预览"
              class="bg-default-100 max-h-48 w-full rounded object-contain"
            />
          </div>
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">分级</h3>
          <div class="flex flex-wrap gap-4 text-sm">
            <label class="flex items-center gap-2">
              <input
                v-model="submitForm.content_limit"
                type="radio"
                value="sfw"
                class="accent-primary"
              />
              SFW
            </label>
            <label class="flex items-center gap-2">
              <input
                v-model="submitForm.content_limit"
                type="radio"
                value="nsfw"
                class="accent-primary"
              />
              NSFW
            </label>
            <span class="text-default-300">|</span>
            <label class="flex items-center gap-2">
              <input
                v-model="submitForm.age_limit"
                type="radio"
                value="all"
                class="accent-primary"
              />
              全年龄
            </label>
            <label class="flex items-center gap-2">
              <input
                v-model="submitForm.age_limit"
                type="radio"
                value="r18"
                class="accent-primary"
              />
              R18
            </label>
          </div>
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">原始语言</h3>
          <select
            v-model="submitForm.original_language"
            class="border-default/20 bg-background w-full rounded-lg border p-2 text-sm"
          >
            <option value="ja-jp">日本語</option>
            <option value="zh-cn">简体中文</option>
            <option value="zh-tw">繁體中文</option>
            <option value="en-us">English</option>
          </select>
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">别名（可选）</h3>
          <KunInput v-model="submitForm.aliases" placeholder="别名1, 别名2, 别名3" />
          <p class="text-default-500 text-xs">英文逗号分隔。</p>
        </section>

        <div
          v-if="submitError"
          class="border-danger/30 bg-danger/10 text-danger rounded-lg border p-3 text-sm"
        >
          {{ submitError }}
        </div>

        <div class="flex justify-end gap-2">
          <KunButton variant="bordered" :disabled="submitting" @click="mode = 'search'">
            返回搜索
          </KunButton>
          <KunButton
            color="primary"
            :loading="submitting"
            :disabled="submitting"
            @click="handleSubmit"
          >
            提交审核
          </KunButton>
        </div>
      </div>
    </KunCard>
  </div>
</template>
