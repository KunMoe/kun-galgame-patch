<script setup lang="ts">
// "Select-or-create" picker for tag / official / engine (handbook §15.1: the
// publish/edit wizard MUST let users pick an existing one OR create a new one
// for original/doujin works VNDB lacks — POST /tag|/official|/engine, any
// logged-in user). Emits the selected id list (replace-all semantics on the
// Wiki side). docs/galgame_wiki/04-taxonomy.md.

import type {
  WikiTag,
  WikiOfficial,
  WikiEngine
} from '~/composables/useGalgameEdit'

type Kind = 'tag' | 'official' | 'engine'
interface Picked {
  id: number
  name: string
}

const props = defineProps<{
  kind: Kind
  modelValue: number[]
  // Optional resolved labels for the initial ids (so chips show names, not #id)
  initial?: Picked[]
  label?: string
}>()
const emit = defineEmits<{ 'update:modelValue': [number[]] }>()

const ge = useGalgameEdit()

const TITLE: Record<Kind, string> = {
  tag: '标签 Tag',
  official: '开发商 Official',
  engine: '引擎 Engine'
}
const CATEGORIES: Record<Kind, { value: string; label: string }[]> = {
  tag: [
    { value: 'content', label: '内容 content' },
    { value: 'sexual', label: '性相关 sexual' },
    { value: 'technical', label: '技术 technical' }
  ],
  official: [
    { value: 'company', label: '公司 company' },
    { value: 'individual', label: '个人 individual' },
    { value: 'amateur', label: '同人 amateur' }
  ],
  engine: []
}

// Selected chips (id + name). Seed from `initial`; ids the parent doesn't
// have names for show as "#id" (backfilled later from search results).
const seedFrom = (ids: number[]): Picked[] =>
  ids.map(
    (id) =>
      picked.value?.find((p) => p.id === id) ??
      props.initial?.find((p) => p.id === id) ?? { id, name: `#${id}` }
  )
const picked = ref<Picked[]>([])
picked.value = seedFrom(props.modelValue)

// Re-seed if the parent sets modelValue AFTER mount (e.g. detail resolved on
// client-side navigation). Guard against the echo from our own sync().
watch(
  () => props.modelValue,
  (nv) => {
    const cur = picked.value.map((p) => p.id)
    if (nv.length === cur.length && nv.every((x) => cur.includes(x))) return
    picked.value = seedFrom(nv)
  }
)

const sync = () =>
  emit(
    'update:modelValue',
    picked.value.map((p) => p.id)
  )

const add = (p: Picked) => {
  if (picked.value.some((x) => x.id === p.id)) return
  picked.value.push(p)
  sync()
}
const remove = (id: number) => {
  picked.value = picked.value.filter((p) => p.id !== id)
  sync()
}

// ─── Search ───────────────────────────────────────────
const keyword = ref('')
const searching = ref(false)
const results = ref<Picked[]>([])
let timer: ReturnType<typeof setTimeout> | null = null

const runSearch = async () => {
  searching.value = true
  try {
    if (props.kind === 'tag') {
      const res = await ge.tagSearch(keyword.value, undefined, 20)
      results.value =
        res.code === 0
          ? (res.data?.items ?? []).map((t: WikiTag) => ({
              id: t.id,
              name: t.name
            }))
          : []
    } else if (props.kind === 'official') {
      const res = await ge.officialSearch(
        keyword.value,
        undefined,
        undefined,
        20
      )
      results.value =
        res.code === 0
          ? (res.data?.items ?? []).map((o: WikiOfficial) => ({
              id: o.id,
              name: o.name
            }))
          : []
    } else {
      // engine has no search endpoint — list all, filter client-side
      const res = await ge.engineList()
      const all =
        res.code === 0
          ? (res.data ?? []).map((e: WikiEngine) => ({
              id: e.id,
              name: e.name
            }))
          : []
      const kw = keyword.value.trim().toLowerCase()
      results.value = kw
        ? all.filter((e) => e.name.toLowerCase().includes(kw))
        : all.slice(0, 30)
    }
    // Backfill real names onto prefilled chips that are still "#id"
    // (notably engine, which has no `initial` source).
    for (const p of picked.value) {
      if (p.name.startsWith('#')) {
        const hit = results.value.find((r) => r.id === p.id)
        if (hit) p.name = hit.name
      }
    }
  } finally {
    searching.value = false
  }
}

watch(keyword, () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(runSearch, 250)
})
onMounted(runSearch)

// ─── Create new ───────────────────────────────────────
const showCreate = ref(false)
const creating = ref(false)
const form = reactive({
  name: '',
  category: props.kind === 'tag' ? 'content' : 'company',
  description: ''
})

const handleCreate = async () => {
  if (!form.name.trim()) {
    useKunMessage('请填写名称', 'warn')
    return
  }
  creating.value = true
  try {
    let res
    if (props.kind === 'tag') {
      res = await ge.createTag({
        name: form.name.trim(),
        category: form.category,
        description: form.description || undefined
      })
    } else if (props.kind === 'official') {
      res = await ge.createOfficial({
        name: form.name.trim(),
        category: form.category,
        description: form.description || undefined
      })
    } else {
      res = await ge.createEngine({
        name: form.name.trim(),
        description: form.description || undefined
      })
    }
    if (res.code === 0 && res.data) {
      const d = res.data as { id: number; name: string }
      add({ id: d.id, name: d.name })
      useKunMessage('已新建并选用', 'success')
      showCreate.value = false
      form.name = ''
      form.description = ''
      runSearch()
    } else {
      // Wiki returns 400 "已存在同名…" → guide the user to search instead.
      useKunMessage(res.message || '新建失败', 'error')
    }
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="border-default/20 space-y-3 rounded-xl border p-3">
    <div class="flex items-center justify-between">
      <p class="text-foreground text-sm font-semibold">
        {{ label ?? TITLE[kind] }}
      </p>
      <KunButton
        variant="light"
        size="sm"
        @click="showCreate = !showCreate"
      >
        {{ showCreate ? '取消新建' : '没有？新建' }}
      </KunButton>
    </div>

    <!-- Selected chips -->
    <div v-if="picked.length" class="flex flex-wrap gap-2">
      <KunChip
        v-for="p in picked"
        :key="p.id"
        color="primary"
        variant="flat"
        size="sm"
      >
        {{ p.name }}
        <KunButton
          variant="light"
          color="danger"
          size="xs"
          is-icon-only
          aria-label="移除"
          @click="remove(p.id)"
        >
          <KunIcon name="lucide:x" class="size-3" />
        </KunButton>
      </KunChip>
    </div>
    <p v-else class="text-default-400 text-xs">未选择（留空表示不修改）</p>

    <!-- Create form -->
    <div
      v-if="showCreate"
      class="border-default/20 bg-default-50 space-y-2 rounded-lg border p-3"
    >
      <KunInput v-model="form.name" placeholder="名称" size="sm" />
      <KunSelect
        v-if="CATEGORIES[kind].length"
        v-model="form.category"
        :options="CATEGORIES[kind]"
      />
      <KunInput
        v-model="form.description"
        placeholder="描述（可选）"
        size="sm"
      />
      <KunButton
        size="sm"
        :loading="creating"
        @click="handleCreate"
      >
        新建并选用
      </KunButton>
    </div>

    <!-- Search -->
    <KunInput
      v-model="keyword"
      :placeholder="`搜索${TITLE[kind]}…`"
      size="sm"
    />
    <div class="max-h-40 space-y-1 overflow-y-auto">
      <KunLoading v-if="searching" description="搜索中..." />
      <KunButton
        v-for="r in results"
        v-else
        :key="r.id"
        variant="light"
        color="default"
        size="sm"
        full-width
        rounded="lg"
        class-name="justify-between"
        :disabled="picked.some((x) => x.id === r.id)"
        @click="add(r)"
      >
        <span>{{ r.name }}</span>
        <KunIcon
          v-if="picked.some((x) => x.id === r.id)"
          name="lucide:check"
          class="text-success size-4"
        />
      </KunButton>
    </div>
  </div>
</template>
