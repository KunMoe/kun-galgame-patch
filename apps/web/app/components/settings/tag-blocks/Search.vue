<script setup lang="ts">
import { watchDebounced } from '@vueuse/core'

defineProps<{ busy: number }>()

const emit = defineEmits<{
  block: [item: SearchEntityItem, mode: KunTagBlockMode]
}>()

const api = useApi()
const { modeOf } = useTagBlocks()

const keyword = ref('')
const results = ref<SearchEntityItem[]>([])
const searching = ref(false)
const confirming = ref(0)

let latest = 0

const search = async () => {
  const current = ++latest
  const q = keyword.value.trim()
  confirming.value = 0
  if (!q) {
    results.value = []
    searching.value = false
    return
  }
  searching.value = true
  const params = new URLSearchParams({
    keywords: q,
    family: 'tag',
    page: '1',
    limit: '20'
  })
  const res = await api.get<SearchEntityResult>(
    `/search/entity?${params.toString()}`
  )
  if (current !== latest) return
  searching.value = false
  results.value = res.code === 0 ? (res.data?.groups[0]?.items ?? []) : []
}

watchDebounced(keyword, search, { debounce: 300, maxWait: 1000 })

const onPick = (tag: SearchEntityItem) => {
  if (!tag.descendant_count) {
    emit('block', tag, 'tree')
    return
  }
  confirming.value = confirming.value === tag.id ? 0 : tag.id
}
</script>

<template>
  <div class="space-y-2">
    <KunInput
      v-model="keyword"
      size="sm"
      placeholder="搜索要屏蔽的标签，例如 NTR"
      is-clearable
    >
      <template #prefix>
        <KunIcon name="lucide:search" class="text-default-400 size-4" />
      </template>
    </KunInput>

    <p
      v-if="searching"
      class="text-default-400 flex items-center gap-1.5 text-sm"
    >
      <KunIcon name="lucide:loader-circle" class="size-4 animate-spin" />
      正在搜索
    </p>

    <p
      v-else-if="keyword.trim() && !results.length"
      class="text-default-400 text-sm"
    >
      没有找到「{{ keyword.trim() }}」
    </p>

    <KunScrollShadow
      v-else-if="results.length"
      axis="vertical"
      scrollbar="thin"
      class-name="max-h-80"
      content-class="gap-0"
    >
      <div
        v-for="tag in results"
        :key="tag.id"
        class="border-default-100 border-b py-2 last:border-b-0"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="min-w-0">
            <p
              :class="
                cn(
                  'truncate text-sm',
                  tag.is_sexual ? 'text-danger-600' : 'text-default-700'
                )
              "
            >
              {{ getPreferredLanguageText(tag.name) }}
            </p>
            <p class="text-default-400 text-xs">
              {{
                tag.descendant_count
                  ? `含 ${tag.descendant_count} 个子标签`
                  : '没有子标签'
              }}
            </p>
          </div>

          <KunChip
            v-if="modeOf(tag.id)"
            size="xs"
            variant="flat"
            color="danger"
          >
            已屏蔽
          </KunChip>
          <KunButton
            v-else
            size="xs"
            variant="flat"
            color="danger"
            :disabled="busy === tag.id"
            @click="onPick(tag)"
          >
            屏蔽
            <KunIcon
              v-if="tag.descendant_count"
              :name="
                confirming === tag.id
                  ? 'lucide:chevron-up'
                  : 'lucide:chevron-down'
              "
              class="size-3"
            />
          </KunButton>
        </div>

        <div
          v-if="confirming === tag.id && !modeOf(tag.id)"
          class="bg-default-50 mt-2 space-y-3 rounded-lg p-3"
        >
          <GalgameTagBlockCoverage
            :tag-id="tag.id"
            :count="tag.descendant_count ?? 0"
          />
          <div class="flex flex-wrap gap-2">
            <KunButton
              size="xs"
              color="danger"
              :disabled="busy === tag.id"
              @click="emit('block', tag, 'tree')"
            >
              连同子标签一起屏蔽
            </KunButton>
            <KunButton
              size="xs"
              variant="flat"
              :disabled="busy === tag.id"
              @click="emit('block', tag, 'exact')"
            >
              只屏蔽「{{ getPreferredLanguageText(tag.name) }}」
            </KunButton>
          </div>
        </div>
      </div>
    </KunScrollShadow>
  </div>
</template>
