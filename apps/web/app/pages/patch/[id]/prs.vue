<script setup lang="ts">
// Galgame edit-requests (PR) — handbook §15 MANDATORY full proxy.
// Non-creators propose edits via a PR; the creator/admin merges or declines
// (Wiki does field-level auto-rebase, or returns a conflict). Authorization is
// enforced by Wiki; we forward its code+message verbatim.
//
// docs/galgame_wiki/02-revisions-and-prs.md §PR (编辑请求).

import type {
  GalgamePR,
  GalgamePRDetail,
  GalgameEditFields
} from '~/composables/useGalgameEdit'
import type { KunUIColor } from '@kun/ui/app/components/kun/ui/type'

const route = useRoute()
const gid = computed(() => Number(route.params.id))
const ge = useGalgameEdit()
const api = useApi()
const userStore = useUserStore()

const page = ref(1)
const limit = 20

const { data, pending, refresh } = await useAsyncData<{
  items: GalgamePR[]
  total: number
}>(
  () => `gal-prs-${gid.value}-${page.value}`,
  async () => {
    const res = await ge.listPRs(gid.value, { page: page.value, limit })
    if (res.code !== 0) return { items: [], total: 0 }
    return { items: res.data?.items ?? [], total: res.data?.total ?? 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [page] }
)
const totalPage = computed(() =>
  Math.max(1, Math.ceil((data.value?.total ?? 0) / limit))
)

const PR_STATUS: Record<number, { text: string; color: KunUIColor }> = {
  0: { text: '待处理', color: 'warning' },
  1: { text: '已合并', color: 'success' },
  2: { text: '已拒绝', color: 'danger' }
}

// ─── Submit PR form ───────────────────────────────────
const showForm = ref(false)
const submitting = ref(false)
const form = reactive({
  name_zh_cn: '',
  name_ja_jp: '',
  name_en_us: '',
  name_zh_tw: '',
  intro_zh_cn: '',
  aliasesText: '',
  note: ''
})
// Associations follow the SAME presence/replace-all semantics as
// PUT /galgame/:gid (docs/galgame_wiki/02 §PR + 01 §presence note): a PR that
// touches tag_ids must carry the WHOLE set, not a delta. So we prefill the
// galgame's current full set (from /patch/:id/detail, surfaced by the
// enricher) and only include a field in the PR when its set actually changed.
const tagIds = ref<number[]>([])
const officialIds = ref<number[]>([])
const engineIds = ref<number[]>([])
const origTagIds = ref<number[]>([])
const origOfficialIds = ref<number[]>([])
const origEngineIds = ref<number[]>([])
const tagInitial = ref<{ id: number; name: string }[]>([])
const officialInitial = ref<{ id: number; name: string }[]>([])
const bannerFile = ref<File | null>(null)

const sameIdSet = (a: number[], b: number[]): boolean => {
  if (a.length !== b.length) return false
  const s = new Set(a)
  return b.every((x) => s.has(x))
}

const { data: galgameDetail } = await useAsyncData<PatchDetail | null>(
  () => `pr-detail-${gid.value}`,
  async () => {
    const res = await api.get<PatchDetail>(`/patch/${gid.value}/detail`)
    return res.code === 0 ? res.data : null
  },
  { default: () => null }
)
watch(
  galgameDetail,
  (d) => {
    if (!d) return
    const tIds = (d.tags ?? []).map((t) => t.id)
    const oIds = (d.officials ?? []).map((o) => o.id)
    const eIds = d.wiki_engine_ids ?? []
    origTagIds.value = tIds
    origOfficialIds.value = oIds
    origEngineIds.value = eIds
    tagIds.value = [...tIds]
    officialIds.value = [...oIds]
    engineIds.value = [...eIds]
    tagInitial.value = (d.tags ?? []).map((t) => ({ id: t.id, name: t.name }))
    officialInitial.value = (d.officials ?? []).map((o) => ({
      id: o.id,
      name: o.name
    }))
  },
  { immediate: true }
)

const buildPayload = (): GalgameEditFields => {
  const p: GalgameEditFields = {}
  if (form.name_zh_cn.trim()) p.name_zh_cn = form.name_zh_cn.trim()
  if (form.name_ja_jp.trim()) p.name_ja_jp = form.name_ja_jp.trim()
  if (form.name_en_us.trim()) p.name_en_us = form.name_en_us.trim()
  if (form.name_zh_tw.trim()) p.name_zh_tw = form.name_zh_tw.trim()
  if (form.intro_zh_cn.trim()) p.intro_zh_cn = form.intro_zh_cn.trim()
  if (form.aliasesText.trim())
    p.aliases = form.aliasesText
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean)
  // Full set, only when changed (never a partial list — would wipe the rest).
  if (!sameIdSet(tagIds.value, origTagIds.value)) p.tag_ids = tagIds.value
  if (!sameIdSet(officialIds.value, origOfficialIds.value))
    p.official_ids = officialIds.value
  if (!sameIdSet(engineIds.value, origEngineIds.value))
    p.engine_ids = engineIds.value
  if (form.note.trim()) p.note = form.note.trim()
  return p
}

const submitPR = async () => {
  if (!userStore.user.id) {
    useKunMessage('请先登录', 'warn')
    return
  }
  const payload = buildPayload()
  if (Object.keys(payload).filter((k) => k !== 'note').length === 0) {
    useKunMessage('请至少修改一个字段', 'warn')
    return
  }
  submitting.value = true
  try {
    const res = bannerFile.value
      ? await ge.submitPRMultipart(gid.value, payload, bannerFile.value)
      : await ge.submitPR(gid.value, payload)
    if (res.code === 0) {
      useKunMessage('已提交编辑请求', 'success')
      showForm.value = false
      page.value = 1
      await refresh()
    } else {
      useKunMessage(res.message || '提交失败', 'error')
    }
  } finally {
    submitting.value = false
  }
}

// ─── PR detail / merge / decline ──────────────────────
const modalOpen = ref(false)
const modalLoading = ref(false)
const detail = ref<GalgamePRDetail | null>(null)
const acting = ref(false)

const openPR = async (prId: number) => {
  modalOpen.value = true
  modalLoading.value = true
  detail.value = null
  const res = await ge.getPR(gid.value, prId)
  modalLoading.value = false
  if (res.code === 0) detail.value = res.data
  else useKunMessage(res.message || '加载 PR 失败', 'error')
}

const doMerge = async () => {
  if (!detail.value) return
  acting.value = true
  try {
    const res = await ge.mergePR(gid.value, detail.value.pr.id)
    if (res.code === 0) {
      useKunMessage('已合并', 'success')
      modalOpen.value = false
      await refresh()
    } else {
      // code 10 = 字段冲突 (base_revision 落后) — surface Wiki's message.
      useKunMessage(res.message || '合并失败', 'error')
    }
  } finally {
    acting.value = false
  }
}
const doDecline = async () => {
  if (!detail.value) return
  const ok = await useKunAlert({
    title: '拒绝编辑请求',
    message: '确定拒绝该编辑请求？'
  })
  if (!ok) return
  acting.value = true
  try {
    const res = await ge.declinePR(gid.value, detail.value.pr.id)
    if (res.code === 0) {
      useKunMessage('已拒绝', 'success')
      modalOpen.value = false
      await refresh()
    } else useKunMessage(res.message || '操作失败', 'error')
  } finally {
    acting.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h2 class="text-lg font-semibold">编辑请求 (PR)</h2>
      <KunButton size="sm" @click="showForm = !showForm">
        {{ showForm ? '收起' : '提交编辑请求' }}
      </KunButton>
    </div>

    <!-- Submit form -->
    <KunCard v-if="showForm" :bordered="true">
      <div class="space-y-3 p-4">
        <p class="text-default-500 text-xs">
          仅需填写要修改的字段，留空表示不改。提交后由创建者 / 管理员审核合并。
        </p>
        <KunInput v-model="form.name_zh_cn" label="名称（简体中文）" />
        <KunInput v-model="form.name_ja_jp" label="名称（日本語）" />
        <KunInput v-model="form.name_en_us" label="名称（English）" />
        <KunInput v-model="form.name_zh_tw" label="名称（繁體中文）" />
        <KunTextarea
          v-model="form.intro_zh_cn"
          label="简介（简体中文）"
          :rows="4"
        />
        <KunInput
          v-model="form.aliasesText"
          label="别名（逗号分隔，替换全部）"
          placeholder="别名1, 别名2"
        />
        <p class="text-default-500 text-xs">
          下方已预填该作品当前的全部标签 / 会社 / 引擎，请在此基础上增删
          （PR 同样按整体集合替换，不能只提交新增项）。
        </p>
        <GalgameEditTaxonomyPicker
          v-model="tagIds"
          kind="tag"
          :initial="tagInitial"
        />
        <GalgameEditTaxonomyPicker
          v-model="officialIds"
          kind="official"
          :initial="officialInitial"
        />
        <GalgameEditTaxonomyPicker
          v-model="engineIds"
          kind="engine"
        />
        <KunFileInput
          v-model="bannerFile"
          accept="image/jpeg,image/png,image/webp"
          :max-size="10 * 1024 * 1024"
          hint="新 Banner（可选）"
          trigger-text="选择新 Banner"
          trigger-icon="lucide:image-plus"
          trigger-size="sm"
          @error-pick="useKunMessage($event, 'error')"
        />
        <KunTextarea
          v-model="form.note"
          label="PR 说明"
          :rows="2"
          placeholder="简述这次修改的内容与理由"
        />
        <div class="flex justify-end">
          <KunButton :loading="submitting" @click="submitPR">
            提交编辑请求
          </KunButton>
        </div>
      </div>
    </KunCard>

    <KunLoading v-if="pending" description="加载中..." />
    <KunNull
      v-else-if="!data?.items?.length"
      description="暂无编辑请求"
    />

    <div v-else class="space-y-2">
      <KunCard
        v-for="pr in data.items"
        :key="pr.id"
        :bordered="true"
        :is-hoverable="true"
        clickable
        content-class="flex w-full items-center justify-between gap-3 p-4"
        @click="openPR(pr.id)"
      >
        <div class="contents">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-mono text-sm font-semibold">PR #{{ pr.id }}</span>
              <KunChip :color="PR_STATUS[pr.status]?.color" size="sm">
                {{ PR_STATUS[pr.status]?.text }}
              </KunChip>
              <span class="text-default-500 text-xs">
                base #{{ pr.base_revision }}
              </span>
            </div>
            <p v-if="pr.note" class="text-default-700 mt-1 truncate text-sm">
              {{ pr.note }}
            </p>
            <p class="text-default-500 mt-1 text-xs">
              用户 #{{ pr.user_id }} ·
              {{ formatDate(pr.created, { isPrecise: true, isShowYear: true }) }}
            </p>
          </div>
          <KunIcon name="lucide:chevron-right" class="size-4 shrink-0" />
        </div>
      </KunCard>

      <KunPagination
        v-if="totalPage > 1"
        v-model:current-page="page"
        :total-page="totalPage"
        :is-loading="pending"
        class="mt-4"
      />
    </div>

    <!-- PR detail modal -->
    <KunModal v-model="modalOpen" :is-show-close-button="true">
      <div class="max-h-[80vh] w-[92vw] max-w-2xl overflow-y-auto p-5">
        <KunLoading v-if="modalLoading" description="加载中..." />
        <template v-else-if="detail">
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <h3 class="text-lg font-semibold">PR #{{ detail.pr.id }}</h3>
            <KunChip :color="PR_STATUS[detail.pr.status]?.color" size="sm">
              {{ PR_STATUS[detail.pr.status]?.text }}
            </KunChip>
          </div>
          <p v-if="detail.pr.note" class="text-default-700 mb-4 text-sm">
            {{ detail.pr.note }}
          </p>

          <GalgameEditDiffView
            :changed-keys="detail.changed_keys"
            :new-snap="detail.pr.snapshot"
            :proposal-only="true"
          />

          <div
            v-if="detail.pr.status === 0"
            class="mt-5 flex justify-end gap-2"
          >
            <KunButton
              variant="bordered"
              color="danger"
              :loading="acting"
              @click="doDecline"
            >
              拒绝
            </KunButton>
            <KunButton
              color="success"
              :loading="acting"
              @click="doMerge"
            >
              合并
            </KunButton>
          </div>
          <p class="text-default-400 mt-4 text-xs">
            合并 / 拒绝仅创建者或管理员可操作；若 base 落后会自动 rebase，冲突时会提示具体字段。
          </p>
        </template>
      </div>
    </KunModal>
  </div>
</template>
