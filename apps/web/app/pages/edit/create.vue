<script setup lang="ts">
useKunSeoMeta({
  title: '发布补丁',
  description: '为 Galgame Wiki 中已有的游戏发布补丁资源'
})

const api = useApi()
const store = useCreatePatchStore()

const errors = ref<Record<string, string>>({})
const submitting = ref(false)

// Aligns with backend vndbIDRegex in apps/api/internal/patch/handler.go.
// `v` + one-or-more digits; no upper bound (VNDB is 5+ digits today).
const VNDBRegex = /^v\d+$/

const config = useRuntimeConfig()
const wikiOrigin = computed(
  () =>
    ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
    'https://galgame.kungal.com'
)

// When the backend reports the vndb_id is not in Wiki yet (code 44001), we
// surface a CTA pointing to the Wiki create page with the id pre-filled,
// instead of just showing a "请先在 Wiki 创建" toast.
const wikiMissingFor = ref<string | null>(null)
const wikiCreateUrl = computed(() =>
  wikiMissingFor.value
    ? `${wikiOrigin.value}/galgame/create?vndb_id=${wikiMissingFor.value}`
    : `${wikiOrigin.value}/galgame/create`
)

const handleSubmit = async () => {
  errors.value = {}
  wikiMissingFor.value = null

  const vndbID = store.data.vndb_id.trim().toLowerCase()
  if (!VNDBRegex.test(vndbID)) {
    useKunMessage('VNDB ID 格式不正确（应为 v + 数字, 如 v19658）', 'error')
    errors.value = { vndb_id: '格式不正确' }
    return
  }

  submitting.value = true
  try {
    const res = await api.post<{ id: number }>('/patch', { vndb_id: vndbID })

    if (res.code === 0 && res.data?.id) {
      store.resetData()
      useKunMessage('发布完成, 正在跳转到游戏页面', 'success')
      await navigateTo(`/patch/${res.data.id}/introduction`)
      return
    }

    // 44001 = ErrWikiGalgameNotFound. The vndb_id is well-formed but the
    // game is not yet in Wiki. Show inline CTA to Wiki's create page.
    if (res.code === 44001) {
      wikiMissingFor.value = vndbID
      useKunMessage(res.message || 'Galgame Wiki 中尚未收录该游戏', 'warn')
    } else {
      useKunMessage(res.message || '发布失败', 'error')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div
    class="mx-auto my-6 flex w-96 max-w-5xl flex-1 items-center justify-center md:w-full"
  >
    <form class="mx-auto w-full flex-1" @submit.prevent="handleSubmit">
      <KunCard :bordered="true">
        <template #header>
          <div class="flex flex-col gap-2 px-1 pt-1">
            <h1 class="text-2xl">发布补丁</h1>
            <p class="text-default-500">
              本站通过 VNDB ID 关联到
              <a
                :href="wikiOrigin"
                target="_blank"
                rel="noopener noreferrer"
                class="text-primary hover:underline"
              >
                Galgame Wiki
              </a>
              。请先确保对应游戏已在 Wiki 中存在，再在本站发布其补丁。
            </p>
            <NuxtLink
              to="/about/notice/galgame-tutorial"
              class="text-primary hover:underline"
            >
              如何在鲲 Galgame 补丁发布 Galgame
            </NuxtLink>
            <div
              class="bg-success/10 border-success/30 rounded-lg border p-3 text-sm"
            >
              <p class="text-success font-bold">仅需 VNDB ID 即可发布</p>
              <p class="text-default-600 mt-1">
                游戏名称、封面、介绍、标签、会社等信息由 Galgame Wiki 统一聚合，
                无需在本站重复填写。
              </p>
            </div>
          </div>
        </template>

        <div class="mt-4 space-y-10">
          <EditCreateVNDBInput :errors="errors.vndb_id" />

          <!-- 44001 CTA: vndb_id not yet on Wiki -->
          <div
            v-if="wikiMissingFor"
            class="bg-warning/10 border-warning/30 space-y-3 rounded-lg border p-3 text-sm"
          >
            <p class="text-warning-700 font-semibold">
              Galgame Wiki 还没有收录
              <strong>{{ wikiMissingFor }}</strong>
            </p>
            <p class="text-default-600">
              本站不直接创建游戏条目，请先到 Galgame Wiki
              发布该游戏，然后回到本页面提交即可。
            </p>
            <a
              :href="wikiCreateUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="block"
            >
              <KunButton color="warning" variant="flat" full-width>
                <KunIcon name="lucide:external-link" class="size-4" />
                前往 Galgame Wiki 创建 {{ wikiMissingFor }}
              </KunButton>
            </a>
          </div>

          <KunButton
            type="submit"
            color="primary"
            full-width
            :loading="submitting"
            :disabled="submitting"
          >
            提交
          </KunButton>
        </div>
      </KunCard>
    </form>
  </div>
</template>
