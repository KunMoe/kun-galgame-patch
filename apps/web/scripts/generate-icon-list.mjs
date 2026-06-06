#!/usr/bin/env node
/**
 * Scans the app (apps/web/app) AND the shared UI layer (packages/ui/app) for
 * icon names used in `KunIcon` components, reads each icon's SVG body from the
 * installed `@iconify-json/*` data in node_modules, and writes the HARDCODED
 * registry consumed by KunIcon:
 *
 *   packages/ui/app/components/kun/icon/icons.ts
 *     export const KUN_ICON_VIEWBOX = '0 0 24 24'
 *     export const KUN_ICONS: Record<string, string> = {
 *       'lucide:menu': '<path .../>', ...
 *     }
 *
 * WHY hardcode instead of @nuxt/icon: @nuxt/icon resolves icons at runtime —
 * it inlines them during SSR but client-fetches any not in the precomputed
 * client bundle, so server and client markup can differ and Vue 3.5 discards +
 * re-renders the mismatched subtree (a hydration "double load"). Inlining every
 * used icon as a static <svg> makes SSR and client emit identical markup with
 * no network round-trip — no @nuxt/icon, no hydration mismatch. KunIcon renders
 * straight from this map, so the `<KunIcon name="lucide:x">` API is unchanged
 * (incl. the dynamic `:name="var"` call sites, which a registry — not per-icon
 * components — is the only way to support).
 *
 * The full icon data lives in node_modules (`@iconify-json/<collection>/
 * icons.json`); this script copies just the used bodies into the committed
 * registry. Re-run after changing icon usage:
 *
 *   npm run icons
 *
 * Both static (`name="lucide:x"`) and ternary (`:name="cond ? 'a:b' : 'c:d'"`)
 * and map-value (`icon: 'lucide:x'`) usages are picked up. Names computed from
 * variables at runtime can't be found statically — add them to MANUAL_ICONS.
 */

import { readFile, readdir, mkdir, writeFile } from 'node:fs/promises'
import { readFileSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const __dirname = dirname(fileURLToPath(import.meta.url))
const ROOT = join(__dirname, '..') // apps/web
const LAYER = join(ROOT, '..', '..', 'packages', 'ui', 'app')
const SCAN_DIRS = [join(ROOT, 'app'), LAYER]
const OUTPUT_FILE = join(
  ROOT,
  '..',
  '..',
  'packages',
  'ui',
  'app',
  'components',
  'kun',
  'icon',
  'icons.ts'
)
const PACKAGE_JSONS = [
  join(ROOT, 'package.json'),
  join(ROOT, '..', '..', 'packages', 'ui', 'package.json')
]

const SCAN_EXTENSIONS = new Set(['.vue', '.ts', '.tsx', '.js', '.mjs'])
const SKIP_DIRS = new Set(['node_modules', '.nuxt', '.output', 'dist', '.git'])

// Icons referenced indirectly (e.g. through props passed at runtime) that the
// regex below cannot find. Add manually if you spot a missing icon at runtime.
const MANUAL_ICONS = []

// Icon references are always quoted string literals — `name="lucide:x"`,
// `:name="cond ? 'a:b' : 'c:d'"`, or map values like `icon: 'lucide:x'`.
const ICON_PATTERN = /["']([a-z][a-z0-9-]{1,30}):([a-z0-9][a-z0-9-]{0,60})["']/g

// Tailwind variant prefixes (and the Vue `update:` event) also look like
// `prefix:rest`; listed only to keep the "missing collection" warning quiet.
const NON_ICON_PREFIXES = new Set([
  'sm', 'md', 'lg', 'xl', '2xl', 'dark', 'light', 'hover', 'focus', 'active',
  'visited', 'target', 'focus-within', 'focus-visible', 'group-hover',
  'group-focus', 'peer-hover', 'peer-focus', 'peer-checked', 'disabled',
  'enabled', 'checked', 'indeterminate', 'default', 'required', 'valid',
  'invalid', 'placeholder-shown', 'autofill', 'read-only', 'empty', 'open',
  'first', 'last', 'only', 'odd', 'even', 'motion-safe', 'motion-reduce',
  'contrast-more', 'print', 'portrait', 'landscape', 'rtl', 'ltr', 'before',
  'after', 'placeholder', 'file', 'marker', 'selection', 'backdrop', 'has',
  'not', 'group', 'peer', 'aria', 'data', 'supports', 'min', 'max', 'update'
])

// Read the installed @iconify-json/* packages from the app + layer manifests.
const installedCollections = async () => {
  const set = new Set()
  for (const p of PACKAGE_JSONS) {
    let pkg
    try {
      pkg = JSON.parse(await readFile(p, 'utf8'))
    } catch {
      continue
    }
    const deps = { ...pkg.dependencies, ...pkg.devDependencies }
    for (const dep of Object.keys(deps)) {
      const m = dep.match(/^@iconify-json\/(.+)$/)
      if (m) set.add(m[1])
    }
  }
  return set
}

// Load an @iconify-json/<collection> dataset from node_modules, resolving the
// package dir via require.resolve so pnpm's nested store layout works.
const collectionCache = new Map()
const loadCollection = (collection) => {
  if (collectionCache.has(collection)) return collectionCache.get(collection)
  let data = null
  try {
    const pkgJson = require.resolve(`@iconify-json/${collection}/package.json`)
    data = JSON.parse(readFileSync(join(dirname(pkgJson), 'icons.json'), 'utf8'))
  } catch {
    data = null
  }
  collectionCache.set(collection, data)
  return data
}

async function* walk(dir) {
  let entries
  try {
    entries = await readdir(dir, { withFileTypes: true })
  } catch {
    return
  }
  for (const entry of entries) {
    if (SKIP_DIRS.has(entry.name)) continue
    const full = join(dir, entry.name)
    if (entry.isDirectory()) {
      yield* walk(full)
    } else if (entry.isFile()) {
      const dot = entry.name.lastIndexOf('.')
      if (dot >= 0 && SCAN_EXTENSIONS.has(entry.name.slice(dot))) {
        yield full
      }
    }
  }
}

async function main() {
  const known = await installedCollections()
  const found = new Set(MANUAL_ICONS)
  const missingDeps = new Map()
  let scanned = 0

  for (const dir of SCAN_DIRS) {
    for await (const file of walk(dir)) {
      scanned++
      const content = await readFile(file, 'utf8')
      for (const match of content.matchAll(ICON_PATTERN)) {
        const [, collection, icon] = match
        if (/^\d+$/.test(icon)) continue // domain strings like 'galgame:1207'
        const name = `${collection}:${icon}`
        if (known.has(collection)) {
          found.add(name)
        } else if (!NON_ICON_PREFIXES.has(collection)) {
          if (!missingDeps.has(collection)) missingDeps.set(collection, new Set())
          missingDeps.get(collection).add(name)
        }
      }
    }
  }

  const names = [...found].sort()

  // Resolve each name to its SVG body + per-icon viewBox from node_modules.
  const entries = [] // [name, viewBox, body]
  const notFound = []
  const transformAliases = []
  for (const name of names) {
    const [collection, icon] = name.split(':')
    const data = loadCollection(collection)
    if (!data) {
      notFound.push(name)
      continue
    }
    let def = data.icons?.[icon]
    if (!def) {
      // Resolve a renamed icon via the collection's alias map. lucide reshuffled
      // many names (circle-x <-> x-circle) and keeps the old name in `aliases`
      // pointing at a `parent` in `icons`.
      const alias = data.aliases?.[icon]
      if (alias?.parent && data.icons?.[alias.parent]) {
        if (alias.hFlip || alias.vFlip || alias.rotate) {
          // None today; flag loudly if one appears so its transform is handled.
          transformAliases.push(name)
        }
        def = data.icons[alias.parent]
      }
    }
    if (!def || typeof def.body !== 'string') {
      notFound.push(name)
      continue
    }
    const w = def.width ?? data.width ?? 24
    const h = def.height ?? data.height ?? 24
    const left = def.left ?? 0
    const top = def.top ?? 0
    entries.push([name, `${left} ${top} ${w} ${h}`, def.body])
  }

  const out = render(entries)
  await mkdir(dirname(OUTPUT_FILE), { recursive: true })
  await writeFile(OUTPUT_FILE, out)

  const rel = relative(join(ROOT, '..', '..'), OUTPUT_FILE)
  console.log(
    `Scanned ${scanned} files (app + layer); wrote ${entries.length} hardcoded icons to ${rel}`
  )

  if (transformAliases.length) {
    console.warn(
      `\n⚠ Alias icons carry a flip/rotate transform that this generator does NOT apply (they'll render untransformed): ${transformAliases.join(', ')}`
    )
  }
  if (notFound.length) {
    console.warn(
      `\n⚠ ${notFound.length} referenced icon(s) not found in the installed collection data (skipped): ${notFound.join(', ')}`
    )
  }
  if (missingDeps.size) {
    console.warn(
      '\n⚠ Referenced icons from NOT-installed collections (install @iconify-json/<collection> or remove the usage):'
    )
    for (const [collection, set] of [...missingDeps].sort()) {
      console.warn(`  @iconify-json/${collection}: ${[...set].sort().join(', ')}`)
    }
  }
}

function render(entries) {
  const lines = entries
    .map(
      ([name, viewBox, body]) =>
        `  '${name}': { v: '${viewBox}', b: ${JSON.stringify(body)} }`
    )
    .join(',\n')
  return `/**
 * Auto-generated by scripts/generate-icon-list.mjs — DO NOT edit by hand.
 * Run \`npm run icons\` after changing <KunIcon name="..."> usage.
 *
 * Hardcoded SVG data for every icon the app + @kun/ui layer render, copied from
 * node_modules @iconify-json/* so icons are inlined statically (no @nuxt/icon
 * runtime, no hydration "double load"). KunIcon wraps each body in
 * <svg :viewBox="v">. \`v\` = viewBox, \`b\` = inner SVG markup.
 */
export interface KunIconData {
  v: string
  b: string
}

export const KUN_ICONS: Record<string, KunIconData> = {
${lines}
}
`
}

main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
