<script setup lang="ts">
// Renders a Wiki revision/PR diff: for each key in `changedKeys`, show the
// old value vs the new value. Snapshots are an open shape (vndb_id, name_*,
// intro_*, aliases, tag_ids, official_ids, engine_ids, links, ...) so values
// are formatted generically. See docs/galgame_wiki/02-revisions-and-prs.md.

import type { GalgameSnapshot } from '~/composables/useGalgameEdit'

const props = defineProps<{
  changedKeys: Record<string, boolean>
  oldSnap?: GalgameSnapshot
  newSnap?: GalgameSnapshot
  // PR detail (GET /galgame/:gid/prs/:id) returns only changed_keys + the
  // proposed snapshot, with no "old" baseline — render a single 提案值 column
  // instead of a misleading empty 旧 column. (02-revisions-and-prs.md)
  proposalOnly?: boolean
}>()

const KEY_LABEL: Record<string, string> = {
  vndb_id: 'VNDB ID',
  name_en_us: '名称 (English)',
  name_ja_jp: '名称 (日本語)',
  name_zh_cn: '名称 (简体中文)',
  name_zh_tw: '名称 (繁體中文)',
  intro_en_us: '简介 (English)',
  intro_ja_jp: '简介 (日本語)',
  intro_zh_cn: '简介 (简体中文)',
  intro_zh_tw: '简介 (繁體中文)',
  content_limit: '内容分级',
  age_limit: '年龄分级',
  original_language: '原始语言',
  aliases: '别名',
  tag_ids: '标签',
  official_ids: '开发商',
  engine_ids: '引擎',
  links: '链接',
  series_id: '系列',
  banner_image_hash: 'Banner'
}

const label = (k: string) => KEY_LABEL[k] ?? k

const fmt = (v: unknown): string => {
  if (v === null || v === undefined || v === '') return '（空）'
  if (Array.isArray(v)) {
    if (v.length === 0) return '（空）'
    return v
      .map((x) =>
        typeof x === 'object' && x !== null ? JSON.stringify(x) : String(x)
      )
      .join('、')
  }
  if (typeof v === 'object') return JSON.stringify(v, null, 2)
  return String(v)
}

const keys = computed(() =>
  Object.keys(props.changedKeys).filter((k) => props.changedKeys[k])
)
</script>

<template>
  <div class="space-y-3">
    <KunNull
      v-if="!keys.length"
      description="与上一版本相比没有字段变化"
    />
    <div
      v-for="k in keys"
      :key="k"
      class="border-default/20 rounded-xl border p-3"
    >
      <p class="text-foreground mb-2 text-sm font-semibold">{{ label(k) }}</p>
      <div v-if="proposalOnly" class="border-primary/30 bg-primary/5 rounded-lg border p-2">
        <p class="text-primary mb-1 text-xs font-medium">提案值</p>
        <pre
          class="text-default-700 text-xs break-words whitespace-pre-wrap"
          >{{ fmt(newSnap?.[k]) }}</pre
        >
      </div>
      <div v-else class="grid gap-2 sm:grid-cols-2">
        <div class="border-danger/30 bg-danger/5 rounded-lg border p-2">
          <p class="text-danger mb-1 text-xs font-medium">旧</p>
          <pre
            class="text-default-600 text-xs break-words whitespace-pre-wrap"
            >{{ fmt(oldSnap?.[k]) }}</pre
          >
        </div>
        <div class="border-success/30 bg-success/5 rounded-lg border p-2">
          <p class="text-success mb-1 text-xs font-medium">新</p>
          <pre
            class="text-default-700 text-xs break-words whitespace-pre-wrap"
            >{{ fmt(newSnap?.[k]) }}</pre
          >
        </div>
      </div>
    </div>
  </div>
</template>
