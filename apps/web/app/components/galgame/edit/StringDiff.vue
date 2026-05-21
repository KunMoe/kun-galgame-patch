<script setup lang="ts">
// String diff (MOYU-PR6 / M4) — line-level LCS with shared-edge trimming,
// optional inline char-level for modified line pairs. Falls back to a plain
// side-by-side block render when DP cells would exceed the safety cap (very
// large pathological inputs).

import {
  diffChars,
  diffLines,
  trimSharedEdges,
  type DiffOp
} from '~/utils/lcs-diff'

const props = defineProps<{
  old: string | null | undefined
  new: string | null | undefined
}>()

const oldS = computed(() => props.old ?? '')
const newS = computed(() => props.new ?? '')

const trimmed = computed(() => trimSharedEdges(oldS.value, newS.value))

// When LCS exceeds the cell cap, diffLines returns null → fall back to a
// plain old/new side-by-side block. The whole component degrades gracefully.
const lineOps = computed<DiffOp[] | null>(() =>
  diffLines(trimmed.value.oldMid, trimmed.value.newMid)
)

// Pair consecutive (del, add) lines so we can inline-char-diff the modified
// line. Standalone del / add stay as solo rows.
type Row =
  | { kind: 'eq'; text: string }
  | { kind: 'del'; text: string }
  | { kind: 'add'; text: string }
  | { kind: 'mod'; oldText: string; newText: string }

const rows = computed<Row[]>(() => {
  if (!lineOps.value) return []
  const r: Row[] = []
  const ops = lineOps.value
  for (let i = 0; i < ops.length; i++) {
    const op = ops[i]
    if (
      op.op === 'del' &&
      ops[i + 1] &&
      ops[i + 1].op === 'add'
    ) {
      r.push({ kind: 'mod', oldText: op.text, newText: ops[i + 1].text })
      i++ // consumed the add
      continue
    }
    if (op.op === 'eq') r.push({ kind: 'eq', text: op.text })
    else if (op.op === 'del') r.push({ kind: 'del', text: op.text })
    else r.push({ kind: 'add', text: op.text })
  }
  return r
})

// Inline char-diff for a modified pair. Returns null on cell-cap overflow.
const charDiffOf = (oldT: string, newT: string) => diffChars(oldT, newT)

const showEqContext = ref(false)
</script>

<template>
  <div class="border-default/20 rounded-lg border p-2 text-xs">
    <!-- Fallback: too big to diff → side-by-side -->
    <div v-if="!lineOps" class="grid gap-2 sm:grid-cols-2">
      <div class="border-danger/30 bg-danger/5 rounded border p-2">
        <p class="text-danger mb-1 font-medium">旧</p>
        <pre class="text-default-600 break-words whitespace-pre-wrap">{{ oldS }}</pre>
      </div>
      <div class="border-success/30 bg-success/5 rounded border p-2">
        <p class="text-success mb-1 font-medium">新</p>
        <pre class="text-default-700 break-words whitespace-pre-wrap">{{ newS }}</pre>
      </div>
    </div>

    <div v-else>
      <p v-if="trimmed.prefix || trimmed.suffix" class="text-default-400 mb-1 italic">
        (已隐藏前后未变内容；下方仅展示真正改动的中间段)
      </p>

      <div v-if="trimmed.oldMid === trimmed.newMid" class="text-default-500 italic">
        (无任何字符层面的差异)
      </div>

      <div v-else class="font-mono leading-5">
        <template v-for="(row, idx) in rows" :key="idx">
          <!-- Equal context: hidden by default to keep diffs focused. -->
          <div
            v-if="row.kind === 'eq'"
            class="text-default-400 px-2"
          >
            <span v-if="showEqContext">{{ row.text }}</span>
            <span v-else class="italic">··· {{ row.text.length }} char unchanged ···</span>
          </div>
          <div
            v-else-if="row.kind === 'del'"
            class="border-danger/30 bg-danger/10 text-danger border-l-2 px-2"
          >
            − {{ row.text }}
          </div>
          <div
            v-else-if="row.kind === 'add'"
            class="border-success/30 bg-success/10 text-success border-l-2 px-2"
          >
            + {{ row.text }}
          </div>
          <!-- Modified pair: inline char diff if affordable, else two rows. -->
          <template v-else>
            <div class="border-danger/30 bg-danger/5 border-l-2 px-2">
              <span class="text-danger mr-1">−</span>
              <template v-for="(seg, si) in charDiffOf(row.oldText, row.newText) || []" :key="'o'+si">
                <span
                  v-if="seg.op !== 'add'"
                  :class="seg.op === 'del' ? 'bg-danger/30 text-danger font-semibold' : 'text-default-600'"
                  >{{ seg.text }}</span
                >
              </template>
              <span v-if="!charDiffOf(row.oldText, row.newText)" class="text-default-600">{{ row.oldText }}</span>
            </div>
            <div class="border-success/30 bg-success/5 border-l-2 px-2">
              <span class="text-success mr-1">+</span>
              <template v-for="(seg, si) in charDiffOf(row.oldText, row.newText) || []" :key="'n'+si">
                <span
                  v-if="seg.op !== 'del'"
                  :class="seg.op === 'add' ? 'bg-success/30 text-success font-semibold' : 'text-default-700'"
                  >{{ seg.text }}</span
                >
              </template>
              <span v-if="!charDiffOf(row.oldText, row.newText)" class="text-default-700">{{ row.newText }}</span>
            </div>
          </template>
        </template>
      </div>

      <KunButton
        v-if="rows.some((r) => r.kind === 'eq')"
        variant="light"
        color="primary"
        size="xs"
        class-name="mt-2 self-start"
        @click="showEqContext = !showEqContext"
      >
        {{ showEqContext ? '隐藏未变上下文' : '展开未变上下文' }}
      </KunButton>
    </div>
  </div>
</template>
