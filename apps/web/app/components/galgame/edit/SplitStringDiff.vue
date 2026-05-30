<script setup lang="ts">
// Split-view string diff — left column = the FULL old text, right column =
// the FULL new text, each highlighted per-line so the user can spot what
// changed in context.
//
// This is the "split / side-by-side" diff mode users expect (GitHub split
// view, VS Code merge editor, etc.) — the previous inline unified
// StringDiff was removed in favour of this layout per user request: each
// changed field shows its FULL old content on the left and FULL new
// content on the right, with field name appearing in each column's
// header so the row is self-describing.
//
// Highlight rules per column:
//   left  — `del` lines and the OLD half of `mod` pairs get red background
//   right — `add` lines and the NEW half of `mod` pairs get green background
//   both  — `eq` lines render plain (full context, never collapsed — the
//           whole point is "see the entire field side-by-side")
//
// Fallback: when diffLines hits the cell-cap (pathological huge inputs),
// we render plain old/new with no highlights — the split is still useful.

import { diffLines, type DiffOp } from '~/utils/lcs-diff'

const props = defineProps<{
  old: string | null | undefined
  new: string | null | undefined
}>()

const oldS = computed(() => props.old ?? '')
const newS = computed(() => props.new ?? '')

// diffLines returns null on cell-cap overflow; we degrade to plain split.
const lineOps = computed<DiffOp[] | null>(() =>
  diffLines(oldS.value, newS.value)
)

// Walk the ops twice (once per column), tagging each line's role so the
// template can paint backgrounds without re-computing. `mod` pairs are
// kept as (del + add) standalone rows here — the split layout already
// separates them visually so we don't need the paired char-diff that
// StringDiff uses for the inline view.
type Line = { kind: 'eq' | 'del' | 'add'; text: string }
const oldLines = computed<Line[]>(() => {
  if (!lineOps.value) return oldS.value.split('\n').map((t) => ({ kind: 'eq' as const, text: t }))
  const out: Line[] = []
  for (const op of lineOps.value) {
    if (op.op === 'add') continue // add lines only appear on the right
    out.push({ kind: op.op === 'del' ? 'del' : 'eq', text: op.text })
  }
  return out
})
const newLines = computed<Line[]>(() => {
  if (!lineOps.value) return newS.value.split('\n').map((t) => ({ kind: 'eq' as const, text: t }))
  const out: Line[] = []
  for (const op of lineOps.value) {
    if (op.op === 'del') continue // del lines only appear on the left
    out.push({ kind: op.op === 'add' ? 'add' : 'eq', text: op.text })
  }
  return out
})
</script>

<template>
  <div class="grid gap-2 sm:grid-cols-2">
    <!-- LEFT — old -->
    <div
      class="border-danger/30 bg-danger/5 max-h-[420px] overflow-y-auto rounded-lg border p-2 font-mono text-xs leading-5"
    >
      <div
        v-for="(l, i) in oldLines"
        :key="'o' + i"
        :class="
          l.kind === 'del'
            ? 'bg-danger/15 text-danger border-danger/40 -mx-2 border-l-2 px-2'
            : 'text-default-700'
        "
      >
        <span v-if="l.kind === 'del'" class="text-danger mr-1">−</span>
        <span class="break-words whitespace-pre-wrap">{{ l.text || '\u00A0' }}</span>
      </div>
      <p v-if="!oldS" class="text-default-400 italic">（空）</p>
    </div>

    <!-- RIGHT — new -->
    <div
      class="border-success/30 bg-success/5 max-h-[420px] overflow-y-auto rounded-lg border p-2 font-mono text-xs leading-5"
    >
      <div
        v-for="(l, i) in newLines"
        :key="'n' + i"
        :class="
          l.kind === 'add'
            ? 'bg-success/15 text-success border-success/40 -mx-2 border-l-2 px-2'
            : 'text-default-700'
        "
      >
        <span v-if="l.kind === 'add'" class="text-success mr-1">+</span>
        <span class="break-words whitespace-pre-wrap">{{ l.text || '\u00A0' }}</span>
      </div>
      <p v-if="!newS" class="text-default-400 italic">（空）</p>
    </div>
  </div>
</template>
