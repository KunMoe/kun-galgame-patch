<script setup lang="ts">
// Array set-level diff (MOYU-PR6 / M9) — for fields like aliases / tag_ids /
// official_ids / engine_ids / links / covers / screenshots where order
// doesn't matter to the reader, "+ added, − removed, = kept" is far clearer
// than a per-index LCS strip. Elements are stringified for the set ops so
// objects (covers/screenshots/links rows) collapse to their JSON form.

import { setDiff } from '~/utils/lcs-diff'

const props = defineProps<{
  old: unknown
  new: unknown
  // K-PR 2026-05-22: optional id → display-name map for *_ids arrays
  // (tag_ids / official_ids / engine_ids / series_id). When provided, we
  // render each id as its display name; missing keys (entity deleted
  // Wiki-side) fall back to `已删除 #<id>`. For non-id arrays (aliases,
  // links, covers, screenshots) the parent omits this prop and we keep
  // the JSON-shape display.
  nameMap?: Record<string, string>
}>()

// Normalize each row to a string key for set diff; preserve original for display.
const toKey = (x: unknown): string => {
  if (x === null || x === undefined) return ''
  if (typeof x === 'object') return JSON.stringify(x)
  return String(x)
}
const display = (x: unknown): string => {
  if (x === null || x === undefined) return '（空）'
  // *_ids array branch: parent passed a names map keyed by id → name.
  // Look up; fall back to `已删除 #<id>` per Wiki spec when the entity is
  // gone (id present in the snapshot but not in the names map).
  if (props.nameMap && (typeof x === 'number' || typeof x === 'string')) {
    const key = String(x)
    const name = props.nameMap[key]
    return name ?? `已删除 #${key}`
  }
  if (typeof x === 'object') {
    // Compact common object shapes for readability.
    const o = x as Record<string, unknown>
    if (typeof o.image_hash === 'string') {
      return `hash:${String(o.image_hash).slice(0, 8)}…(sort:${o.sort_order ?? '?'})`
    }
    if (typeof o.name === 'string' && typeof o.link === 'string') {
      return `${o.name} → ${o.link}`
    }
    return JSON.stringify(x)
  }
  return String(x)
}

const oldArr = computed<unknown[]>(() =>
  Array.isArray(props.old) ? (props.old as unknown[]) : []
)
const newArr = computed<unknown[]>(() =>
  Array.isArray(props.new) ? (props.new as unknown[]) : []
)

const diff = computed(() => {
  // Compute set diff on stringified keys, then map back to representative
  // originals from new/old for display.
  const oldKeys = oldArr.value.map(toKey)
  const newKeys = newArr.value.map(toKey)
  const { added, removed, kept } = setDiff(oldKeys, newKeys)
  const byKeyNew = new Map<string, unknown>()
  newArr.value.forEach((x, i) => byKeyNew.set(newKeys[i]!, x))
  const byKeyOld = new Map<string, unknown>()
  oldArr.value.forEach((x, i) => byKeyOld.set(oldKeys[i]!, x))
  return {
    added: added.map((k) => byKeyNew.get(k) ?? k),
    removed: removed.map((k) => byKeyOld.get(k) ?? k),
    kept: kept.map((k) => byKeyOld.get(k) ?? byKeyNew.get(k) ?? k)
  }
})
</script>

<template>
  <div class="border-default/20 space-y-1 rounded-lg border p-2 text-xs">
    <div v-if="diff.added.length">
      <span class="text-success font-medium">+ 新增 ({{ diff.added.length }})：</span>
      <span class="text-success">
        <span
          v-for="(v, i) in diff.added"
          :key="'a' + i"
          class="bg-success/10 mr-1 inline-block rounded px-1 py-0.5"
        >
          {{ display(v) }}
        </span>
      </span>
    </div>
    <div v-if="diff.removed.length">
      <span class="text-danger font-medium">− 删除 ({{ diff.removed.length }})：</span>
      <span class="text-danger">
        <span
          v-for="(v, i) in diff.removed"
          :key="'r' + i"
          class="bg-danger/10 mr-1 inline-block rounded px-1 py-0.5"
        >
          {{ display(v) }}
        </span>
      </span>
    </div>
    <div v-if="diff.kept.length" class="text-default-500">
      = 保留 {{ diff.kept.length }} 项不变
    </div>
    <div
      v-if="!diff.added.length && !diff.removed.length && !diff.kept.length"
      class="text-default-400 italic"
    >
      (空集合)
    </div>
  </div>
</template>
