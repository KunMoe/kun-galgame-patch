import { kunMoyuMoe } from '~/config/moyu-moe'

// resolveBannerUrl picks the best URL for a galgame banner.
//
// Wiki PR5 (2026-05-18) replaced `banner_image_hash` with the derived
// `effective_banner_hash` (= the image_hash of covers[sort_order=0], or empty
// if no pinned cover). During the transition we still fall back to the legacy
// absolute `banner` URL when no hash is present.
//
// Resolution order: effective_banner_hash → legacy banner URL → ''.
//
// The hash → URL convention follows image_service §02 (`{cdn}/ab/cd/<hash>.webp`,
// no `/img/` segment; shard prefix = first 2 + next 2 hex chars). The `mini` variant maps to the
// pre-generated 460×259 galgame_banner thumbnail; for legacy URLs we keep the
// existing `-mini.avif` substitution that PatchCard / Card / ranking pages
// historically use.

// Accepts either:
//   - the wiki-side galgame object directly (`{effective_banner_hash, banner}`)
//   - the patch row (`{banner, galgame: {effective_banner_hash, banner}}`)
// Whichever shape the caller has at hand. Hash field is preferred from nested
// galgame, then top-level; legacy URL preferred from top-level patch.banner,
// then nested galgame.banner (they're the same after enricher copy, this is
// just resilience).
type BannerSource = {
  effective_banner_hash?: string | null
  banner?: string | null
  galgame?: {
    effective_banner_hash?: string | null
    banner?: string | null
  } | null
}

const IMAGE_BED = kunMoyuMoe.domain.imageBed.replace(/\/$/, '')

const isHexHash = (s: string): boolean => /^[0-9a-f]{4,}$/i.test(s)

const buildImageServiceURL = (hash: string, variant?: string): string => {
  if (hash.length < 4 || !isHexHash(hash)) return ''
  const shard1 = hash.slice(0, 2)
  const shard2 = hash.slice(2, 4)
  const suffix = variant ? `_${variant}` : ''
  return `${IMAGE_BED}/${shard1}/${shard2}/${hash}${suffix}.webp`
}

// Public helper for any place that has just an image_service hash and wants
// the CDN URL (covers/screenshots editor thumbnails, etc.). Same convention
// as the avatar/banner resolvers: returns '' for empty/malformed hash so the
// caller can show a placeholder.
export const imageServiceUrl = (hash: string, variant?: string): string =>
  buildImageServiceURL(hash, variant)

// Legacy banner URLs were `*.avif`; the historical thumbnail pattern is
// `<name>-mini.avif`. Caller passes variant='mini' to opt in.
const toLegacyVariant = (url: string, variant?: 'mini'): string => {
  if (!variant) return url
  return url.replace(/\.avif$/, `-${variant}.avif`)
}

// galgame may be the embedded Wiki object (PatchDetail.galgame) or a list-row
// shape that exposes the same two fields. Anywhere `patch.banner` was used
// historically, replace with `resolveBannerUrl(patch.galgame, variant) || patch.banner`
// — both fallbacks already accounted for so the trailing `|| patch.banner` is
// only a safety net for objects lacking the nested galgame shape.
export const resolveBannerUrl = (
  source: BannerSource | null | undefined,
  variant?: 'mini'
): string => {
  if (!source) return ''
  const hash = (
    source.galgame?.effective_banner_hash ??
    source.effective_banner_hash ??
    ''
  ).trim()
  if (hash) {
    const u = buildImageServiceURL(hash, variant)
    if (u) return u
  }
  const legacy = (source.banner ?? source.galgame?.banner ?? '').trim()
  if (!legacy) return ''
  return toLegacyVariant(legacy, variant)
}
