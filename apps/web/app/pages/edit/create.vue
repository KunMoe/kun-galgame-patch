<script setup lang="ts">

useKunDisableSeo('发布 Galgame')

const api = useApi()
const route = useRoute()
const userStore = useUserStore()

type WizardMode = 'search' | 'submit'
const mode = ref<WizardMode>('search')

const searchQuery = ref('')
const searching = ref(false)
const searched = ref(false)
interface GalgameName {
  id: number
  name_en_us: string
  name_ja_jp: string
  name_zh_cn: string
  name_zh_tw: string
  display_name?: string
  latin?: string
}
interface GalgameHit extends GalgameName {
  claim_state: string
  vndb_id: string
  banner: string
  effective_banner_hash?: string
}
interface PendingHit {
  id: number
  display_name: string
  claim_state: string
  reason?: string
}
interface SearchResult {
  items: GalgameHit[]
  pending: PendingHit[]
  total: number
}
const results = ref<SearchResult>({ items: [], pending: [], total: 0 })

const displayName = (h: GalgameName): string =>
  h.name_zh_cn ||
  h.name_zh_tw ||
  h.name_ja_jp ||
  h.name_en_us ||
  h.display_name ||
  h.latin ||
  `#${h.id}`

const claimStateLabel = (state: string): string =>
  state === 'declined' ? '已拒绝（可重新提交）' : '审核中'

const isPendingReview = (h: GalgameHit): boolean => h.claim_state === 'pending'

const gameHref = (h: GalgameHit): string => `/galgame/${h.id}?tab=resource`

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
      results.value = {
        items: res.data?.items ?? [],
        pending: res.data?.pending ?? [],
        total: res.data?.total ?? 0
      }
      searched.value = true
    } else {
      useKunMessage(res.message || '搜索失败', 'error')
    }
  } finally {
    searching.value = false
  }
}

onMounted(() => {
  const q = String(route.query.q ?? '').trim()
  if (!q) return
  searchQuery.value = q
  if (userStore.isLoggedIn) doSearch()
})

const goToMine = async () => {
  await navigateTo('/me/submissions')
}

interface SubmitForm {
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

const startSubmit = () => {
  const q = searchQuery.value.trim()
  const looksLikeVndbOrReleaseId = /^[vr]\d+$/i.test(q)
  if (q && !looksLikeVndbOrReleaseId && !submitForm.name_zh_cn && !submitForm.name_ja_jp) {
    if (/[一-龥]/.test(q)) {
      submitForm.name_zh_cn = q
    } else if (/[぀-ゟ゠-ヿ]/.test(q)) {
      submitForm.name_ja_jp = q
    } else {
      submitForm.name_en_us = q
    }
  }
  mode.value = 'submit'
}

const submitting = ref(false)
const submitError = ref<string | null>(null)
const submitKey = useSubmitKey()

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

  const payload: Record<string, unknown> = {
    content_limit: submitForm.content_limit,
    age_limit: submitForm.age_limit
  }
  if (submitForm.name_zh_cn.trim()) payload.name_zh_cn = submitForm.name_zh_cn.trim()
  if (submitForm.name_zh_tw.trim()) payload.name_zh_tw = submitForm.name_zh_tw.trim()
  if (submitForm.name_ja_jp.trim()) payload.name_ja_jp = submitForm.name_ja_jp.trim()
  if (submitForm.name_en_us.trim()) payload.name_en_us = submitForm.name_en_us.trim()
  if (submitForm.intro_zh_cn.trim()) payload.intro_zh_cn = submitForm.intro_zh_cn.trim()
  if (submitForm.aliases.trim()) payload.aliases = submitForm.aliases.trim()
  if (submitForm.original_language) payload.original_language = submitForm.original_language

  submitting.value = true
  try {
    const res = await api.post<{ id: number; claim_state: string }>(
      '/galgame/submit',
      { ...payload, submit_key: submitKey.keyFor(payload) }
    )
    submitKey.settle(res.code)

    if (res.code === 0) {
      useKunMessage(
        '提交成功！您的作品已进入审核队列，审核通过后将获得 +3 萌萌点',
        'success'
      )
      await navigateTo('/me/submissions')
      return
    }
    submitError.value = res.message || '提交失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AuthRequired>
    <div class="container mx-auto my-4">
    <KunHeader
      name="发布 Galgame"
      description="先搜索您想发布资源的游戏。资料库里已有的作品直接打开详情页发布资源；确实没有的再新建申请。"
    />
    <div class="mx-auto max-w-3xl">

    <div v-if="mode === 'search'" class="mt-6 space-y-4">
      <KunCard :bordered="true">
        <div class="space-y-3 p-4">
          <h2 class="text-lg font-semibold">1. 搜索资料库</h2>
          <p class="text-default-500 text-sm">
            搜索覆盖资料库中的全部游戏。打开详情页即可发布资源；catalog 没有的原创 / 同人作品走下方新建申请。
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
              class-name="shrink-0"
              :loading="searching"
              :disabled="searching || !searchQuery.trim()"
            >
              搜索
            </KunButton>
          </form>
        </div>
      </KunCard>

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
                <p class="font-semibold">{{ hit.display_name || `#${hit.id}` }}</p>
                <p class="text-default-500 text-xs">
                  {{ claimStateLabel(hit.claim_state) }}
                  <span v-if="hit.reason"> · {{ hit.reason }}</span>
                </p>
              </div>
              <KunButton variant="bordered" size="sm" @click="goToMine">
                查看进度
              </KunButton>
            </div>
          </div>
        </div>
      </KunCard>

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
                </p>
              </div>
              <span
                v-if="isPendingReview(hit)"
                class="text-default-400 shrink-0 text-sm"
              >
                他人投稿审核中
              </span>
              <KunButton
                v-else
                color="primary"
                size="sm"
                :href="gameHref(hit)"
              >
                查看 / 发布资源
              </KunButton>
            </div>
          </div>
        </div>
      </KunCard>

      <KunCard
        v-if="searched && results.items.length === 0 && results.pending.length === 0"
        :bordered="true"
      >
        <div class="space-y-3 p-4 text-center">
          <p class="text-default-500">没有找到匹配的条目</p>
          <KunButton color="primary" @click="startSubmit">
            提交新作到资料库
          </KunButton>
        </div>
      </KunCard>

      <div v-if="searched" class="text-center">
        <KunButton variant="light" color="primary" @click="startSubmit">
          以上都不是？提交新作
        </KunButton>
      </div>
    </div>

    <KunCard v-else :bordered="true" class-name="mt-6">
      <div class="space-y-4 p-4">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold">2. 提交新作到资料库</h2>
          <KunButton variant="light" size="sm" @click="mode = 'search'">
            <KunIcon name="lucide:arrow-left" class="size-4" />
            回到搜索
          </KunButton>
        </div>

        <div class="border-default/20 bg-default-50 rounded-lg border p-3 text-sm">
          <p class="text-default-700">
            提交后状态为「审核中」，进入审核队列。审核通过后您将获得
            <strong>+3 萌萌点</strong>，并可在本站发布该游戏的补丁。
            <br />
            封面请在条目通过审核后，到条目页的「编辑」中补充。
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

        <div class="border-default/20 bg-default-50 rounded-lg border p-3 text-xs text-default-600">
          这个表单只用于 <strong>VNDB 还没有收录</strong> 的作品（原创、同人、冷门等），
          所以不需要填 VNDB ID。
          <br />
          已经在 VNDB 有条目的游戏，一般已被资料库自动同步，请优先回到上一步用
          <strong>名字或 VNDB ID 搜索</strong>，搜到后打开详情页发布资源即可。
          <br />
          如果你确定它在 VNDB 有条目却怎么都搜不到，多半是刚收录、资料库还没同步过来，
          可以先核对一下 ID、过一两天再来搜；不必在这里把 VNDB ID 当作名字填进去。
        </div>

        <section class="space-y-2">
          <h3 class="font-semibold">简介（简体中文）</h3>
          <KunMarkdownEditor
            :model-value="submitForm.intro_zh_cn"
            :image="false"
            @update:model-value="(val) => (submitForm.intro_zh_cn = val)"
          />
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">分级</h3>
          <div class="flex flex-wrap items-center gap-6 text-sm">
            <KunRadioGroup
              v-model="submitForm.content_limit"
              orientation="horizontal"
              :options="[
                { value: 'sfw', label: 'SFW' },
                { value: 'nsfw', label: 'NSFW' }
              ]"
            />
            <span class="text-default-300">|</span>
            <KunRadioGroup
              v-model="submitForm.age_limit"
              orientation="horizontal"
              :options="[
                { value: 'all', label: '全年龄' },
                { value: 'r18', label: 'R18' }
              ]"
            />
          </div>
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">原始语言</h3>
          <KunSelect
            v-model="submitForm.original_language"
            :options="[
              { value: 'ja-jp', label: '日本語' },
              { value: 'zh-cn', label: '简体中文' },
              { value: 'zh-tw', label: '繁體中文' },
              { value: 'en-us', label: 'English' }
            ]"
          />
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
    </div>
  </AuthRequired>
</template>
