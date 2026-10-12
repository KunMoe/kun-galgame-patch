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
type BannerMeta = {
  effective_banner_hash?: string | null
  banner?: string | null
  effective_banner_thumbhash?: string | null
}

type BannerSource = BannerMeta & {
  galgame?: BannerMeta | null
}

type PortraitMeta = {
  effective_portrait_hash?: string | null
  effective_portrait_thumbhash?: string | null
}

type PortraitSource = PortraitMeta & {
  galgame?: PortraitMeta | null
}

type CoverMeta = BannerMeta & PortraitMeta

type CoverSource = CoverMeta & {
  galgame?: CoverMeta | null
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

// imageAspectRatio turns intrinsic dims into a CSS aspect-ratio string for
// KunImage's `aspect-ratio` prop. Falls back to the historical 16/9 box when
// dims are unknown (metadata absent / pre-backfill) so layout stays stable and
// portrait covers stop being cropped only once we actually know their shape.
export const imageAspectRatio = (
  width?: number | null,
  height?: number | null,
  fallback = '16 / 9'
): string => (width && height ? `${width} / ${height}` : fallback)

export const resolveBannerThumbhash = (
  source: BannerSource | null | undefined
): string | undefined =>
  (
    source?.galgame?.effective_banner_thumbhash ??
    source?.effective_banner_thumbhash ??
    ''
  ).trim() || undefined

// resolvePortraitUrl reads catalog's tall cover slot off the same dual-shape
// source. It returns '' rather than falling back to the banner: a caller that
// wants "portrait, else banner" should say so, since the two want different
// crops.
//
// It takes no variant, unlike resolveBannerUrl: image_service picks the preset
// from what the image IS, and a cover's `mini` is the 460x259 galgame_banner
// crop — asking for it would hand back a landscape 460x259 for a portrait
// 850x1080, with no error to notice. (Character art is a different preset and
// its `mini` does stay portrait, 256x360.)
export const resolvePortraitUrl = (
  source: PortraitSource | null | undefined
): string => {
  const hash = (
    source?.galgame?.effective_portrait_hash ??
    source?.effective_portrait_hash ??
    ''
  ).trim()
  return hash ? buildImageServiceURL(hash) : ''
}

// resolveMaker reads the credited company off the same dual-shape source the
// cover resolvers take: the enriched rows nest it under `galgame`, the flat
// search hit carries it at the top level.
type MakerSource = {
  maker?: GalgameMaker | null
  galgame?: { maker?: GalgameMaker | null } | null
}

export const resolveMaker = (
  source: MakerSource | null | undefined
): GalgameMaker | undefined =>
  source?.galgame?.maker ?? source?.maker ?? undefined

export const resolvePortraitThumbhash = (
  source: PortraitSource | null | undefined
): string | undefined =>
  (
    source?.galgame?.effective_portrait_thumbhash ??
    source?.effective_portrait_thumbhash ??
    ''
  ).trim() || undefined

// resolveCoverUrl is the cover for a cropped 3/4 header box, and both the intro
// and the resource page read it. The resource page used to read the banner
// slot, which catalog fills with a landscape screenshot when a work has no wide
// cover, and readers took that screenshot for the wrong game's patch. The
// portrait slot is filled from ANY cover when no portrait-shaped one exists, so
// the banner fallback only matters for a work with no cover at all.
export const resolveCoverUrl = (
  source: CoverSource | null | undefined
): string => resolvePortraitUrl(source) || resolveBannerUrl(source)

export const resolveCoverThumbhash = (
  source: CoverSource | null | undefined
): string | undefined =>
  resolvePortraitUrl(source)
    ? resolvePortraitThumbhash(source)
    : resolveBannerThumbhash(source)
