<script setup lang="ts">
// Admin blog management: list (incl. drafts) + create / edit / delete.
// Banner + inline images go through image_service via POST /upload/image-service
// (preset 'topic', the moyu-enabled preset); the banner is stored as a content
// hash, inline images are inserted into the markdown body as CDN URLs.
useKunDisableSeo('博客管理')

const api = useApi()
const userStore = useUserStore()
const config = useRuntimeConfig()
const apiBase = config.public.apiBase as string

const list = ref<BlogCard[]>([])
const total = ref(0)
const page = ref(1)
const limit = 12
const pending = ref(false)
const totalPages = computed(() => Math.ceil(total.value / limit))

const load = async () => {
  pending.value = true
  const res = await api.get<{ items: BlogCard[]; total: number }>(
    `/admin/blog?page=${page.value}&limit=${limit}`
  )
  pending.value = false
  if (res.code === 0) {
    list.value = res.data.items ?? []
    total.value = res.data.total ?? 0
  } else {
    useKunMessage(res.message || '加载失败', 'error')
  }
}

// ─── editor modal ──────────────────────────────────────
const modalOpen = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const bannerUploading = ref(false)
const inlineUploading = ref(false)

const form = reactive({
  title: '',
  summary: '',
  content: '',
  banner_image_hash: '',
  bannerPreview: '',
  status: 0,
  pin: false
})

const statusOptions = [
  { value: 0, label: '草稿' },
  { value: 1, label: '已发布' }
]

const resetForm = () => {
  form.title = ''
  form.summary = ''
  form.content = ''
  form.banner_image_hash = ''
  form.bannerPreview = ''
  form.status = 0
  form.pin = false
}

const openCreate = () => {
  resetForm()
  editingId.value = null
  modalOpen.value = true
}

const openEdit = async (id: number) => {
  const res = await api.get<BlogEdit>(`/admin/blog/${id}`)
  if (res.code !== 0) {
    useKunMessage(res.message || '加载失败', 'error')
    return
  }
  const b = res.data
  form.title = b.title
  form.summary = b.summary
  form.content = b.content
  form.banner_image_hash = b.banner_image_hash
  form.bannerPreview = b.banner
  form.status = b.status
  form.pin = b.pin
  editingId.value = id
  modalOpen.value = true
}

// ─── image_service upload ──────────────────────────────
const uploadImage = async (
  file: File
): Promise<{ hash: string; url: string } | null> => {
  const fd = new FormData()
  fd.append('preset', 'topic')
  fd.append('file', file, file.name)
  try {
    const r = await $fetch<{
      code: number
      message: string
      data: { hash: string; url: string } | null
    }>(`${apiBase}/upload/image-service`, {
      method: 'POST',
      body: fd,
      credentials: 'include'
    })
    if (r.code === 0 && r.data) return { hash: r.data.hash, url: r.data.url }
    useKunMessage(r.message || '图片上传失败', 'error')
    return null
  } catch {
    useKunMessage('图片上传失败', 'error')
    return null
  }
}

const onBannerFile = async (e: Event) => {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  bannerUploading.value = true
  const res = await uploadImage(f)
  bannerUploading.value = false
  ;(e.target as HTMLInputElement).value = ''
  if (res) {
    form.banner_image_hash = res.hash
    form.bannerPreview = res.url
  }
}

const onInlineFile = async (e: Event) => {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  inlineUploading.value = true
  const res = await uploadImage(f)
  inlineUploading.value = false
  ;(e.target as HTMLInputElement).value = ''
  if (res) form.content += `\n\n![](${res.url})\n`
}

const clearBanner = () => {
  form.banner_image_hash = ''
  form.bannerPreview = ''
}

// ─── save / delete ─────────────────────────────────────
const save = async () => {
  if (!form.title.trim()) return useKunMessage('请填写标题', 'warn')
  if (!form.content.trim()) return useKunMessage('请填写正文', 'warn')
  saving.value = true
  const body = {
    title: form.title,
    summary: form.summary,
    content: form.content,
    banner_image_hash: form.banner_image_hash,
    status: form.status,
    pin: form.pin
  }
  const res =
    editingId.value === null
      ? await api.post('/admin/blog', body)
      : await api.put(`/admin/blog/${editingId.value}`, body)
  saving.value = false
  if (res.code === 0) {
    useKunMessage(editingId.value === null ? '已创建' : '已保存', 'success')
    modalOpen.value = false
    await load()
  } else {
    useKunMessage(res.message || '保存失败', 'error')
  }
}

const deleteTarget = ref<BlogCard | null>(null)
const deleting = ref(false)
const confirmDelete = async () => {
  if (!deleteTarget.value) return
  deleting.value = true
  const res = await api.delete(`/admin/blog/${deleteTarget.value.id}`)
  deleting.value = false
  if (res.code === 0) {
    useKunMessage('已删除', 'success')
    deleteTarget.value = null
    await load()
  } else {
    useKunMessage(res.message || '删除失败', 'error')
  }
}

const onChangePage = async (v: number) => {
  page.value = v
  await load()
}

const fmtDate = (d: string) => new Date(d).toLocaleDateString('zh-CN')

onMounted(() => {
  if (!userStore.isModerator) {
    useKunMessage('无权限访问', 'error')
    navigateTo('/')
    return
  }
  load()
})
</script>

<template>
  <div class="container mx-auto my-4 space-y-4">
    <div class="flex items-center justify-between">
      <KunHeader name="博客管理" description="创建 / 编辑 / 删除博客文章" />
      <KunButton color="primary" @click="openCreate">
        <KunIcon name="lucide:plus" class="size-4" />
        新建博客
      </KunButton>
    </div>

    <KunLoading v-if="pending" description="加载中..." />
    <KunNull v-else-if="!list.length" description="暂无博客" />

    <div v-else class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="text-default-500 border-default-200 border-b text-left">
          <tr>
            <th class="px-3 py-2">标题</th>
            <th class="px-3 py-2">状态</th>
            <th class="px-3 py-2">置顶</th>
            <th class="px-3 py-2">浏览</th>
            <th class="px-3 py-2">创建</th>
            <th class="px-3 py-2 text-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="b in list"
            :key="b.id"
            class="border-default-100 border-b"
          >
            <td class="max-w-xs truncate px-3 py-2">
              <NuxtLink :to="`/blog/${b.id}`" class="hover:text-primary">
                {{ b.title }}
              </NuxtLink>
            </td>
            <td class="px-3 py-2">
              <KunChip
                :color="b.status === 1 ? 'success' : 'default'"
                variant="flat"
                size="sm"
              >
                {{ b.status === 1 ? '已发布' : '草稿' }}
              </KunChip>
            </td>
            <td class="px-3 py-2">{{ b.pin ? '是' : '—' }}</td>
            <td class="text-default-500 px-3 py-2">{{ b.view }}</td>
            <td class="text-default-500 px-3 py-2">{{ fmtDate(b.created) }}</td>
            <td class="px-3 py-2">
              <div class="flex justify-end gap-2">
                <KunButton size="sm" variant="flat" @click="openEdit(b.id)">
                  编辑
                </KunButton>
                <KunButton
                  size="sm"
                  variant="flat"
                  color="danger"
                  @click="deleteTarget = b"
                >
                  删除
                </KunButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="onChangePage"
      />
    </div>

    <!-- create / edit modal -->
    <KunModal v-model="modalOpen" inner-class-name="max-w-3xl">
      <div class="space-y-4">
        <h2 class="text-xl font-bold">
          {{ editingId === null ? '新建博客' : '编辑博客' }}
        </h2>

        <KunInput v-model="form.title" label="标题" placeholder="博客标题" />
        <KunTextarea
          v-model="form.summary"
          label="摘要"
          placeholder="用于列表卡片 / SEO 的简短描述"
          :rows="2"
        />

        <!-- banner -->
        <div class="space-y-2">
          <p class="text-default-600 text-sm font-medium">封面图</p>
          <img
            v-if="form.bannerPreview"
            :src="form.bannerPreview"
            class="aspect-video w-64 rounded-lg object-cover"
          />
          <div class="flex items-center gap-2">
            <label class="inline-block">
              <input
                type="file"
                accept="image/*"
                class="hidden"
                @change="onBannerFile"
              />
              <span
                class="border-default-300 hover:bg-default-100 inline-flex cursor-pointer items-center gap-1 rounded-lg border px-3 py-1.5 text-sm"
              >
                <KunIcon name="lucide:upload" class="size-4" />
                {{ bannerUploading ? '上传中...' : '上传封面' }}
              </span>
            </label>
            <KunButton
              v-if="form.banner_image_hash"
              size="sm"
              variant="light"
              color="danger"
              @click="clearBanner"
            >
              移除
            </KunButton>
          </div>
        </div>

        <!-- content -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <p class="text-default-600 text-sm font-medium">正文 (Markdown)</p>
            <label class="inline-block">
              <input
                type="file"
                accept="image/*"
                class="hidden"
                @change="onInlineFile"
              />
              <span
                class="border-default-300 hover:bg-default-100 inline-flex cursor-pointer items-center gap-1 rounded-lg border px-2 py-1 text-xs"
              >
                <KunIcon name="lucide:image-plus" class="size-3.5" />
                {{ inlineUploading ? '上传中...' : '插入图片' }}
              </span>
            </label>
          </div>
          <KunTextarea
            v-model="form.content"
            placeholder="支持 Markdown；图片用「插入图片」上传到图床后自动插入"
            :rows="14"
          />
        </div>

        <div class="flex flex-wrap items-center gap-4">
          <div class="w-40">
            <KunSelect
              v-model="form.status"
              :options="statusOptions"
              label="状态"
            />
          </div>
          <KunSwitch v-model="form.pin" label="置顶" />
        </div>

        <div class="flex justify-end gap-2 pt-2">
          <KunButton variant="light" @click="modalOpen = false">取消</KunButton>
          <KunButton color="primary" :loading="saving" @click="save">
            {{ editingId === null ? '创建' : '保存' }}
          </KunButton>
        </div>
      </div>
    </KunModal>

    <!-- delete confirm -->
    <KunModal
      :model-value="!!deleteTarget"
      inner-class-name="max-w-md"
      @update:model-value="(v: boolean) => { if (!v) deleteTarget = null }"
    >
      <div class="space-y-4">
        <h2 class="text-lg font-bold">确认删除</h2>
        <p class="text-default-600 text-sm">
          确定删除博客「{{ deleteTarget?.title }}」吗？此操作不可恢复。
        </p>
        <div class="flex justify-end gap-2">
          <KunButton variant="light" @click="deleteTarget = null">取消</KunButton>
          <KunButton color="danger" :loading="deleting" @click="confirmDelete">
            删除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
