// LCS-based string diff helpers (MOYU-PR6 / M4).
//
// Used by GalgameEditDiffView's StringDiff sub-component to render long intro
// text changes as line-level (and optionally char-level within a modified
// line) rather than whole-block old/new. The Wiki intro fields can reach 20 KB
// per language — a side-by-side block dump there is almost unreadable.
//
// Performance guard: full LCS is O(n·m). Worst case for two 20 KB strings
// split per-line is bounded by line count, not characters, so usually fine.
// But for pathological inputs (one huge unsplit line edited mid-string) the
// caller can drop into char-level mode and that DOES blow up. Cap is checked
// up front; on overflow the caller renders a side-by-side block fallback.

export const STRING_DIFF_DP_MAX_CELLS = 4_000_000

export type DiffOp = { op: 'eq' | 'add' | 'del'; text: string }

/**
 * Strip the common prefix + suffix from `oldS` and `newS` so the LCS body
 * only operates on the genuinely different middle. Strongly accelerates the
 * "only changed one paragraph" case.
 */
export const trimSharedEdges = (
  oldS: string,
  newS: string
): { prefix: string; oldMid: string; newMid: string; suffix: string } => {
  const minLen = Math.min(oldS.length, newS.length)
  let s = 0
  while (s < minLen && oldS[s] === newS[s]) s++
  let e = 0
  // ensure prefix and suffix don't overlap on either side
  while (
    e < minLen - s &&
    oldS[oldS.length - 1 - e] === newS[newS.length - 1 - e]
  )
    e++
  return {
    prefix: oldS.slice(0, s),
    oldMid: oldS.slice(s, oldS.length - e),
    newMid: newS.slice(s, newS.length - e),
    suffix: e > 0 ? oldS.slice(oldS.length - e) : ''
  }
}

/**
 * Generic LCS diff over an arbitrary array of comparable items. Returns null
 * when the DP cell count would exceed STRING_DIFF_DP_MAX_CELLS so the caller
 * can fall back to a non-LCS rendering (e.g. side-by-side block).
 */
const lcsDiff = <T>(a: T[], b: T[], eq: (x: T, y: T) => boolean):
  | { op: 'eq' | 'add' | 'del'; value: T }[]
  | null => {
  const n = a.length
  const m = b.length
  if ((n + 1) * (m + 1) > STRING_DIFF_DP_MAX_CELLS) return null

  // dp[i][j] = LCS length of a[i..] and b[j..]
  const dp: number[][] = Array.from({ length: n + 1 }, () =>
    new Array<number>(m + 1).fill(0)
  )
  // Non-null assertions: the loop bounds (i in [0,n), j in [0,m)) guarantee
  // every index is in range, but `noUncheckedIndexedAccess` widens `a[i]` /
  // `dp[i]` to `… | undefined`. Asserting here keeps the hot loops allocation-
  // free instead of adding per-cell guards.
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      if (eq(a[i]!, b[j]!)) dp[i]![j] = dp[i + 1]![j + 1]! + 1
      else dp[i]![j] = Math.max(dp[i + 1]![j]!, dp[i]![j + 1]!)
    }
  }

  const out: { op: 'eq' | 'add' | 'del'; value: T }[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (eq(a[i]!, b[j]!)) {
      out.push({ op: 'eq', value: a[i]! })
      i++
      j++
    } else if (dp[i + 1]![j]! >= dp[i]![j + 1]!) {
      out.push({ op: 'del', value: a[i]! })
      i++
    } else {
      out.push({ op: 'add', value: b[j]! })
      j++
    }
  }
  while (i < n) {
    out.push({ op: 'del', value: a[i++]! })
  }
  while (j < m) {
    out.push({ op: 'add', value: b[j++]! })
  }
  return out
}

/** Line-level diff. Returns null on DP cell overflow (caller falls back). */
export const diffLines = (oldS: string, newS: string): DiffOp[] | null => {
  const a = oldS.split('\n')
  const b = newS.split('\n')
  const r = lcsDiff(a, b, (x, y) => x === y)
  if (!r) return null
  return r.map((x) => ({ op: x.op, text: x.value }))
}

/**
 * Char-level diff for a single short(-ish) line pair. Same cell-cap guard.
 * Useful inline-within a modified line to highlight which characters actually
 * changed (vs the whole line glowing red+green).
 */
export const diffChars = (oldS: string, newS: string): DiffOp[] | null => {
  const a = Array.from(oldS)
  const b = Array.from(newS)
  const r = lcsDiff(a, b, (x, y) => x === y)
  if (!r) return null
  // Merge consecutive same-op chars into one segment for fewer DOM nodes.
  const merged: DiffOp[] = []
  for (const x of r) {
    const last = merged[merged.length - 1]
    if (last && last.op === x.op) last.text += x.value
    else merged.push({ op: x.op, text: x.value })
  }
  return merged
}

// ─── Array set-level diff (MOYU-PR6 / M9) ──────────────────────────────────
//
// For taxonomy id arrays (tag_ids / official_ids / engine_ids) or aliases the
// per-index LCS view is noise: re-ordering produces N×2 fake ops. Set-level
// diff just answers "what was added, what was removed, what stayed".

export type SetDiff<T> = { added: T[]; removed: T[]; kept: T[] }

export const setDiff = <T>(oldA: readonly T[], newA: readonly T[]): SetDiff<T> => {
  const oldSet = new Set(oldA)
  const newSet = new Set(newA)
  const added: T[] = []
  const removed: T[] = []
  const kept: T[] = []
  for (const x of newA) {
    if (oldSet.has(x)) {
      if (!kept.includes(x)) kept.push(x)
    } else added.push(x)
  }
  for (const x of oldA) {
    if (!newSet.has(x)) removed.push(x)
  }
  return { added, removed, kept }
}
