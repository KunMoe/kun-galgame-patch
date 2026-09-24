<script setup lang="ts">
const props = defineProps<{ patchId: number }>()
const open = defineModel<boolean>({ required: true })
const emits = defineEmits<{ saved: [payload: { favorited: boolean }] }>()

const api = useApi()

const folders = ref<FolderMembership[]>([])
const selected = ref<number[]>([])
const loading = ref(false)
const saving = ref(false)
const creating = ref(false)
const newName = ref('')

const load = async () => {
  loading.value = true
  const res = await api.get<{ folders: FolderMembership[] }>(
    `/patch/${props.patchId}/folder`
  )
  loading.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '读取收藏夹失败', 'error')
    open.value = false
    return
  }
  folders.value = res.data.folders
  selected.value = res.data.folders.filter((f) => f.contains).map((f) => f.id)
}

watch(open, (isOpen) => {
  if (isOpen) load()
})

const toggle = (id: number) => {
  const set = new Set(selected.value)
  if (set.has(id)) {
    set.delete(id)
  } else {
    set.add(id)
  }
  selected.value = [...set]
}

// The default folder is unnamed by design — the backfill wrote empty names and
// every client derives the label — so it is labelled here rather than shown blank.
const labelOf = (folder: Folder) =>
  folder.name.trim() || (folder.is_default ? '默认收藏夹' : '未命名收藏夹')

const createFolder = async () => {
  const name = newName.value.trim()
  if (!name) return
  creating.value = true
  const res = await api.post<Folder>('/folder', {
    name,
    description: '',
    visibility: 'public'
  })
  creating.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '创建收藏夹失败', 'error')
    return
  }
  newName.value = ''
  folders.value = [...folders.value, { ...res.data, contains: false }]
  toggle(res.data.id)
}

const save = async () => {
  saving.value = true
  const res = await api.put<{ favorited: boolean }>(
    `/patch/${props.patchId}/folder`,
    { folder_ids: selected.value }
  )
  saving.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '保存失败', 'error')
    return
  }
  emits('saved', { favorited: selected.value.length > 0 })
  useKunMessage('已保存', 'success')
  open.value = false
}
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-md" aria-label="加入收藏夹">
    <div class="space-y-4">
      <h3 class="text-foreground text-lg font-semibold">加入收藏夹</h3>

      <KunLoading v-if="loading" description="加载中..." />

      <KunNull
        v-else-if="!folders.length"
        description="还没有收藏夹，在下方新建一个吧"
      />

      <div v-else class="max-h-72 space-y-2 overflow-y-auto">
        <label
          v-for="folder in folders"
          :key="folder.id"
          class="border-default-200 hover:bg-default-100 flex cursor-pointer items-center gap-3 rounded-xl border px-3 py-2 transition-colors"
        >
          <KunCheckBox
            :model-value="selected.includes(folder.id)"
            @update:model-value="toggle(folder.id)"
          />
          <KunIcon
            :name="folder.visibility === 'private' ? 'lucide:lock' : 'lucide:globe'"
            class="text-default-400 size-4 shrink-0"
          />
          <div class="min-w-0 flex-1">
            <p class="text-foreground truncate text-sm">{{ labelOf(folder) }}</p>
            <p class="text-default-400 text-xs">{{ folder.item_count }} 个游戏</p>
          </div>
        </label>
      </div>

      <div class="flex items-center gap-2">
        <KunInput
          v-model="newName"
          placeholder="新建收藏夹..."
          class="flex-1"
          @keyup.enter="createFolder"
        />
        <KunButton
          variant="light"
          :loading="creating"
          :disabled="!newName.trim()"
          @click="createFolder"
        >
          新建
        </KunButton>
      </div>

      <div class="flex justify-end gap-3">
        <KunButton variant="light" color="danger" @click="open = false">
          取消
        </KunButton>
        <KunButton color="primary" :loading="saving" @click="save">保存</KunButton>
      </div>
    </div>
  </KunModal>
</template>
