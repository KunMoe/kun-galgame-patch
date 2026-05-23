<script setup lang="ts">
// Renders a Wiki revision/PR diff: for each key in `changedKeys`, show the
// old value vs the new value. Snapshots are an open shape (vndb_id, name_*,
// intro_*, aliases, tag_ids, official_ids, engine_ids, links, ...) so values
// are formatted generically. See docs/galgame_wiki/02-revisions-and-prs.md.

import type {
  GalgameDiffNames,
  GalgameSnapshot
} from '~/composables/useGalgameEdit'

const props = defineProps<{
  changedKeys: Record<string, boolean>
  oldSnap?: GalgameSnapshot
  newSnap?: GalgameSnapshot
  // PR detail (GET /galgame/:gid/prs/:id) returns only changed_keys + the
  // proposed snapshot, with no "old" baseline — render a single 提案值 column
  // instead of a misleading empty 旧 column. (02-revisions-and-prs.md)
  proposalOnly?: boolean
  // K-PR 2026-05-22: taxonomy id → display-name map from the Wiki diff/PR
  // response (see GalgameDiffNames). Routed down to ArrayDiff for the
  // matching field so users see "校园, 治愈" instead of "1, 2".
  names?: GalgameDiffNames
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
  // W2 / Wiki PR5: banner is now expressed via covers[sort_order=0]; the old
  // `banner_image_hash` snapshot key was migrated and dropped Wiki-side.
  effective_banner_hash: '当前 Banner',
  covers: '封面集合',
  screenshots: '截图集合',
  release_date: '发售日期',
  release_date_tba: '发售日期未定'
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

// ─── MOYU-PR6 / M4+M9 — render-dispatch by field shape ──────────────────
// Wiki snapshots mix scalars, long markdown (intro_*), id arrays (tag_ids
// etc), and object arrays (covers / screenshots / links). A one-size block
// dump is unreadable for the long-string and array shapes; route each to a
// purpose-built sub-component.
const ARRAY_KEYS = new Set([
  'aliases',
  'tag_ids',
  'official_ids',
  'engine_ids',
  'links',
  'covers',
  'screenshots'
])
// Long-string threshold; intro_* always counts as long regardless of length.
const LONG_STRING_THRESHOLD = 200

const valOf = (k: string, snap?: GalgameSnapshot): unknown =>
  snap ? snap[k] : undefined

const isArrayKey = (k: string, o: unknown, n: unknown): boolean =>
  ARRAY_KEYS.has(k) || Array.isArray(o) || Array.isArray(n)

const isLongStringKey = (k: string, o: unknown, n: unknown): boolean => {
  if (k.startsWith('intro_')) return true
  const oLen = typeof o === 'string' ? o.length : 0
  const nLen = typeof n === 'string' ? n.length : 0
  if (oLen === 0 && nLen === 0) return false
  return oLen + nLen >= LONG_STRING_THRESHOLD
}

// Field key → which slot of the names map to pass to ArrayDiff. Each
// taxonomy id array maps to one named bucket; non-id arrays (aliases /
// links / covers / screenshots) get no map and keep their JSON-shape
// rendering.
const NAMES_SLOT: Record<string, keyof GalgameDiffNames> = {
  tag_ids: 'tags',
  official_ids: 'officials',
  engine_ids: 'engines',
  series_id: 'series'
}
const nameMapFor = (k: string): Record<string, string> | undefined => {
  const slot = NAMES_SLOT[k]
  return slot ? props.names?.[slot] : undefined
}
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
      <!-- Proposal-only (PR detail has no `old`) — single 提案值 block.
           Field label stays above since there's no split to label. -->
      <template v-if="proposalOnly">
        <p class="text-foreground mb-2 text-sm font-semibold">{{ label(k) }}</p>
        <div class="border-primary/30 bg-primary/5 rounded-lg border p-2">
          <p class="text-primary mb-1 text-xs font-medium">提案值</p>
          <pre
            class="text-default-700 text-xs break-words whitespace-pre-wrap"
            >{{ fmt(newSnap?.[k]) }}</pre
          >
        </div>
      </template>

      <!-- Array fields (aliases / *_ids / links / covers / screenshots) →
           set-level "+ added / − removed / = kept" collapse. This view is
           inherently more useful than a split for arrays (you want to see
           the delta, not two parallel lists), so we keep ArrayDiff here
           with the field label above the way it always was. -->
      <template
        v-else-if="isArrayKey(k, valOf(k, oldSnap), valOf(k, newSnap))"
      >
        <p class="text-foreground mb-2 text-sm font-semibold">{{ label(k) }}</p>
        <GalgameEditArrayDiff
          :old="valOf(k, oldSnap)"
          :new="valOf(k, newSnap)"
          :name-map="nameMapFor(k)"
        />
      </template>

      <!-- Long strings (intro_*, or any string ≥ 200 chars) — split-view
           side-by-side with per-line highlights so the reader sees the
           WHOLE old text on the left and the WHOLE new text on the right,
           with changed lines tinted in place. -->
      <div
        v-else-if="isLongStringKey(k, valOf(k, oldSnap), valOf(k, newSnap))"
        class="grid gap-2 sm:grid-cols-2"
      >
        <div class="text-danger flex items-center gap-2 text-xs font-medium">
          <span class="text-foreground font-semibold">{{ label(k) }}</span>
          <KunChip color="danger" variant="flat" size="sm">旧</KunChip>
        </div>
        <div class="text-success flex items-center gap-2 text-xs font-medium">
          <span class="text-foreground font-semibold">{{ label(k) }}</span>
          <KunChip color="success" variant="flat" size="sm">新</KunChip>
        </div>
        <div class="sm:col-span-2">
          <GalgameEditSplitStringDiff
            :old="(valOf(k, oldSnap) as string | null | undefined) ?? ''"
            :new="(valOf(k, newSnap) as string | null | undefined) ?? ''"
          />
        </div>
      </div>

      <!-- Scalars (vndb_id, release_date, content_limit, short strings, …)
           — side-by-side with field name + 旧/新 chip in each column
           header so the row is self-describing (no separate row title). -->
      <div v-else class="grid gap-2 sm:grid-cols-2">
        <div class="border-danger/30 bg-danger/5 rounded-lg border p-2">
          <div
            class="text-danger mb-1 flex items-center gap-2 text-xs font-medium"
          >
            <span class="text-foreground font-semibold">{{ label(k) }}</span>
            <KunChip color="danger" variant="flat" size="sm">旧</KunChip>
          </div>
          <pre
            class="text-default-700 text-sm break-words whitespace-pre-wrap"
            >{{ fmt(oldSnap?.[k]) }}</pre
          >
        </div>
        <div class="border-success/30 bg-success/5 rounded-lg border p-2">
          <div
            class="text-success mb-1 flex items-center gap-2 text-xs font-medium"
          >
            <span class="text-foreground font-semibold">{{ label(k) }}</span>
            <KunChip color="success" variant="flat" size="sm">新</KunChip>
          </div>
          <pre
            class="text-default-700 text-sm break-words whitespace-pre-wrap"
            >{{ fmt(newSnap?.[k]) }}</pre
          >
        </div>
      </div>
    </div>
  </div>
</template>
