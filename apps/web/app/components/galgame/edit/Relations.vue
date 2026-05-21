<script setup lang="ts">
// Links / aliases / contributors editor (handbook §15, mandatory full proxy).
// Each write auto-creates a Wiki revision; Wiki enforces auth (creator/admin
// for contributor delete; any writer for links/aliases) and we forward its
// code+message. docs/galgame_wiki/03-relations.md.

import type {
  GalgameLink,
  GalgameAlias,
  GalgameContributor
} from '~/composables/useGalgameEdit'

const props = defineProps<{ gid: number }>()
const ge = useGalgameEdit()

const links = ref<GalgameLink[]>([])
const aliases = ref<GalgameAlias[]>([])
const contributors = ref<GalgameContributor[]>([])
const loading = ref(true)

const reload = async () => {
  loading.value = true
  const [l, a, c] = await Promise.all([
    ge.listLinks(props.gid),
    ge.listAliases(props.gid),
    ge.listContributors(props.gid)
  ])
  links.value = l.code === 0 ? (l.data ?? []) : []
  aliases.value = a.code === 0 ? (a.data ?? []) : []
  contributors.value = c.code === 0 ? (c.data ?? []) : []
  loading.value = false
}
onMounted(reload)

// ─── Links ────────────────────────────────────────────
const newLink = reactive({ name: '', link: '' })
const busy = ref(false)

const addLink = async () => {
  if (!newLink.name.trim() || !newLink.link.trim()) {
    useKunMessage('请填写链接名称与地址', 'warn')
    return
  }
  busy.value = true
  try {
    const res = await ge.createLink(props.gid, {
      name: newLink.name.trim(),
      link: newLink.link.trim()
    })
    if (res.code === 0) {
      useKunMessage('已添加链接', 'success')
      newLink.name = ''
      newLink.link = ''
      await reload()
    } else useKunMessage(res.message || '添加失败', 'error')
  } finally {
    busy.value = false
  }
}
const removeLink = async (id: number) => {
  const res = await ge.deleteLink(props.gid, id)
  if (res.code === 0) {
    useKunMessage('已删除', 'success')
    await reload()
  } else useKunMessage(res.message || '删除失败', 'error')
}

// ─── Aliases ──────────────────────────────────────────
const newAlias = ref('')
const addAlias = async () => {
  if (!newAlias.value.trim()) return
  busy.value = true
  try {
    const res = await ge.createAlias(props.gid, newAlias.value.trim())
    if (res.code === 0) {
      useKunMessage('已添加别名', 'success')
      newAlias.value = ''
      await reload()
    } else useKunMessage(res.message || '添加失败', 'error')
  } finally {
    busy.value = false
  }
}
const removeAlias = async (id: number) => {
  const res = await ge.deleteAlias(props.gid, id)
  if (res.code === 0) {
    useKunMessage('已删除', 'success')
    await reload()
  } else useKunMessage(res.message || '删除失败', 'error')
}

// ─── Contributors ─────────────────────────────────────
const removeContributor = async (userId: number) => {
  if (!confirm('确定移除该贡献者？仅创建者或管理员可操作。')) return
  const res = await ge.deleteContributor(props.gid, userId)
  if (res.code === 0) {
    useKunMessage('已移除', 'success')
    await reload()
  } else useKunMessage(res.message || '移除失败', 'error')
}
</script>

<template>
  <div class="space-y-6">
    <KunLoading v-if="loading" description="加载中..." />
    <template v-else>
      <!-- Links -->
      <section class="space-y-3">
        <h3 class="text-foreground text-base font-semibold">链接</h3>
        <div
          v-for="l in links"
          :key="l.id"
          class="border-default/20 flex items-center justify-between gap-2 rounded-lg border p-2"
        >
          <div class="min-w-0">
            <p class="truncate text-sm font-medium">{{ l.name }}</p>
            <a
              :href="l.link"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary truncate text-xs hover:underline"
            >
              {{ l.link }}
            </a>
          </div>
          <KunButton
            variant="light"
            color="danger"
            size="sm"
            @click="removeLink(l.id)"
          >
            删除
          </KunButton>
        </div>
        <div class="flex flex-col gap-2 sm:flex-row">
          <KunInput v-model="newLink.name" placeholder="名称（如 官网）" size="sm" />
          <KunInput
            v-model="newLink.link"
            placeholder="https://..."
            size="sm"
          />
          <KunButton size="sm" :loading="busy" @click="addLink">
            添加
          </KunButton>
        </div>
      </section>

      <!-- Aliases -->
      <section class="space-y-3">
        <h3 class="text-foreground text-base font-semibold">别名</h3>
        <div class="flex flex-wrap gap-2">
          <KunChip
            v-for="a in aliases"
            :key="a.id"
            color="default"
            variant="flat"
            size="sm"
          >
            {{ a.name }}
            <KunButton
              variant="light"
              color="danger"
              size="xs"
              is-icon-only
              aria-label="删除别名"
              @click="removeAlias(a.id)"
            >
              <KunIcon name="lucide:x" class="size-3" />
            </KunButton>
          </KunChip>
          <span v-if="!aliases.length" class="text-default-400 text-xs">
            暂无别名
          </span>
        </div>
        <div class="flex gap-2">
          <KunInput v-model="newAlias" placeholder="新别名" size="sm" />
          <KunButton size="sm" :loading="busy" @click="addAlias">
            添加
          </KunButton>
        </div>
      </section>

      <!-- Contributors -->
      <section class="space-y-3">
        <h3 class="text-foreground text-base font-semibold">贡献者</h3>
        <div
          v-for="c in contributors"
          :key="c.id"
          class="border-default/20 flex items-center justify-between gap-2 rounded-lg border p-2"
        >
          <div class="flex items-center gap-2">
            <img
              v-if="c.user?.avatar"
              :src="c.user.avatar"
              :alt="c.user?.name ?? ''"
              class="bg-default-100 size-7 rounded-full object-cover"
            />
            <span class="text-sm">
              {{ c.user?.name ?? `用户 #${c.user_id}` }}
            </span>
          </div>
          <KunButton
            variant="light"
            color="danger"
            size="sm"
            @click="removeContributor(c.user_id)"
          >
            移除
          </KunButton>
        </div>
        <p v-if="!contributors.length" class="text-default-400 text-xs">
          暂无贡献者
        </p>
      </section>
    </template>
  </div>
</template>
