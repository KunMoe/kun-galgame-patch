// After D8 / D11 / D12 (2026-04-21), patch-related types are significantly slimmed:
//   - cover/screenshot/char/person/release are owned by the Galgame Wiki (D8)
//   - tag/company also belong to Wiki (D11)
//   - patch itself no longer stores name/introduction/banner/released/content_limit/engine/alias (D12)
//
// All JSON keys are snake_case to match the backend wire format exactly.
// The backend enricher merges patch + Wiki galgame into the shape below.

// The extra a list card draws beside the poster: the two credits a reader picks
// a game by, and a short shelf of tags. Every lane that answers a card fills it
// off the same catalog read that built the card.
interface GalgameCardTag {
  id: number
  name: string
}

interface GalgameCardFacet {
  scenario?: KunLanguage[]
  illustration?: KunLanguage[]
  tags?: GalgameCardTag[]
}

interface GalgameCard {
  id: number
  name: KunLanguage
  vndb_id: string
  bangumi_id: number | null
  banner: string
  // Pinned-cover metadata at the TOP level, which is the shape a flat catalog
  // hit hands over (the /galgame/search/publish picker) as opposed to the
  // enriched rows that nest it under `galgame`. resolveBannerUrl /
  // resolveBannerThumbhash read either — nested first, then here — so a card
  // built from a flat hit still gets its cover. Absent on the enriched rows;
  // `banner` alone is not a fallback for it, since the catalog leaves that
  // legacy URL empty.
  effective_banner_hash?: string
  effective_banner_thumbhash?: string
  effective_portrait_hash?: string
  effective_portrait_thumbhash?: string
  maker?: GalgameMaker
  facet?: GalgameCardFacet
  view: number
  download: number
  type: string[]
  language: string[]
  platform: string[]
  content_limit: KunContentLimit
  status: number
  // True when moyu holds a real local patch row (created on a real publish,
  // favorite, or comment — not on view). Absent on card shapes built
  // client-side (e.g. search); treat missing as on-site.
  is_on_forum?: boolean
  // Sticky SEO flag: true after the first accepted resource, not cleared
  // when resources are later deleted. Hidden/ban still unpublishes.
  indexed?: boolean
  created: Date | string
  resource_update_time: Date | string
  // Locally-mirrored wiki galgame.release_date (RFC3339 from backend, or
  // null/absent when unknown). Used to render the release month on cards and
  // make the release-date sort/filter result legible. Format at render.
  release_date?: string | null
  count: {
    favorite_by: number
    contribute_by: number
    resource: number
    comment: number
  }
  // PATCH PUBLISHER (moyu patch.user_id — who registered this galgame on moyu /
  // uploaded its patches). Owner-gating (edit/delete) keys on this. NOT the
  // entry creator — see `creator`.
  user?: KunUser
  // GALGAME ENTRY CREATOR — Galgame Wiki's galgame.user_id (single source of
  // truth, the same id kungal shows), resolved to a user brief by the backend.
  // Display the "词条创建者" position from this; absent when wiki has no
  // creator / lookup miss (fall back to `user`).
  creator?: KunUser
  // Optional: raw Wiki galgame object (includes age_limit, original_language, etc.)
  // U1 (2026-05-18): release_date / release_date_tba replaced the old `released`
  // string. release_date is "YYYY-MM-DD" or null (unknown); release_date_tba=true
  // means 官方已宣布但日期未定 — the two are independent.
  // W2 / Wiki PR5 (2026-05-18): banner_image_hash gone. effective_banner_hash =
  // covers[sort_order=0].image_hash (or '' if no pinned cover). covers /
  // screenshots are arrays of CoverInput / ScreenshotInput per Wiki §03-relations
  // (presence semantics on PUT: omit = keep; [] = clear; non-empty = full replace).
  galgame?: {
    id: number
    vndb_id: string
    name_en_us: string
    name_zh_cn: string
    name_ja_jp: string
    name_zh_tw: string
    banner: string
    effective_banner_hash: string
    // Pinned cover's intrinsic metadata (filled at read time by wiki from
    // image_service; optional until its backfill runs). Drives the card/detail
    // banner's real aspect-ratio + ThumbHash blur-up.
    effective_banner_width?: number
    effective_banner_height?: number
    effective_banner_thumbhash?: string
    // covers.portrait, catalog's tall slot. Catalog fills it from ANY cover when
    // the work has no portrait-shaped one, so it is not reliably taller than it
    // is wide — crop it into the box you want instead of laying out from its
    // aspect ratio. Absent when the work has no cover at all.
    effective_portrait_hash?: string
    effective_portrait_width?: number
    effective_portrait_height?: number
    effective_portrait_thumbhash?: string
    maker?: GalgameMaker
    covers: GalgameCoverRow[]
    screenshots: GalgameScreenshotRow[]
    content_limit: string
    age_limit: string
    original_language: string
    release_date: string | null
    // How to read release_date: day | month | year. release_date is NORMALIZED
    // from the catalog's partial-ISO value, so the two MUST be read together —
    // a "2026-06-01" with precision 'month' means "some day in June", not the
    // 1st. Absent when the work has no dated release.
    release_precision?: GalgameReleasePrecision
    // The catalog's claim VISIBILITY for this work (wave A2-2):
    //   'live'  — a published wiki entry
    //   'draft' — a wiki entry that is not published yet
    //   ''      — no wiki entry claims this work at all
    // 'hidden' (withdrawn) never reaches the frontend: the backend drops those.
    // This replaced the wiki `status` int, which was product state the canonical
    // catalog face does not carry.
    claim_state: string
    // The FORUM's page id for this game, read off the catalog claim. `id` is
    // this site's page and the catalog work in one number; kungal still keys on
    // the legacy gid, so a link to the forum has to use this and nothing else.
    // Absent when no forum page claims the work.
    forum_gid?: number
  }
}

// The company a card credits. Which of a work's companies that is depends on
// what it DID (developer > circle > brand > publisher), decided by the backend;
// the name arrives whole so the 标题语言 setting picks the slot here.
interface GalgameMaker {
  id: number
  name: KunLanguage
}

interface GalgameCoverRow {
  image_hash: string
  sort_order: number
  sexual: number
  violence: number
  source: string
  source_key: string
  // VNDB cover type (covers only): '' | main | pkgfront | dig | pkgback |
  // pkgcontent | pkgside | pkgmed. Optional because screenshots reuse this shape.
  kind?: string
  // Intrinsic image metadata from image_service (optional until backfill).
  // Real aspect-ratio (no portrait crop) + ThumbHash blur-up placeholder.
  width?: number
  height?: number
  thumbhash?: string
}

interface GalgameScreenshotRow extends GalgameCoverRow {
  caption: string
}

// Presets moyu may send in the `preset` form field of POST
// /api/v1/upload/image-service. image_service applies per-preset size + quota
// and rejects a preset not allowlisted for moyu's OAuth client. moyu sends
// exactly one: 'topic' (free-form gallery / editor-inline / doc images).
// Avatars go through OAuth's /auth/me/avatar and never reach this endpoint.
//
// 'galgame_screenshot' left this union in wave 161 (N5) along with the backend
// lane that carried it: it had zero senders here, and the wiki write face it
// proxied to is being taken down. Keep this union in sync with the presets infra
// enables for www.moyu.moe.
type GalgameImageUploadPreset = 'topic'

// Patch header (GET /patch/:id) -- GalgameCard + is_favorite.
interface PatchHeader extends GalgameCard {
  is_favorite: boolean
  // The DLsite purchase entry, assembled server-side. Absent whenever the work
  // has no buyable DLsite id or the feature is unconfigured, which is what the
  // button keys off. dlsite_campaign_name is present only while infra reports a
  // running campaign; without it the coupon link is the standing landing page
  // and keeps its own description.
  dlsite_purchase_url?: string
  dlsite_coupon_url?: string
  dlsite_campaign_name?: string
}

// Patch detail (GET /patch/:id/detail) -- GalgameCard plus the full catalog record.
// introduction_markdown is filled in by the backend via Wiki /galgame/:gid; the
// enricher also resolves tags/officials by name on the server side so the frontend
// can render labels directly.
interface PatchDetailTag {
  id: number
  name: string
  aliases?: string[]
  category: string
  spoiler_level: number
}

interface PatchDetailOfficial {
  id: number
  name: string
  aliases?: string[]
  category: string
  lang: string
  // image_service content hash of the brand logo (wave 170 P3), not a URL.
  // '' / absent = no logo, and the chip stays text-only.
  logo_hash?: string
}

// One credited person. `id` is catalog's NAME id, which is what kungal's
// /galgame/staff/:id takes.
// Entity names arrive on all four slots rather than pre-rendered, so the
// reader's 标题语言 setting picks between them the same way it does for a game
// title. Render with getPreferredLanguageText, never by reading one key.
interface PatchDetailPerson {
  id: number
  name: KunLanguage
}

interface PatchDetailCharacter {
  id: number
  name: KunLanguage
  kind: string
  spoiler: number
  // image_service content hashes, not URLs. `image` is the head shot, `figure`
  // the standing art; either may be absent.
  image_hash?: string
  figure_hash?: string
  voices: PatchDetailPerson[]
}

interface PatchDetailStaffGroup {
  role_key: string
  role_name: string
  people: (PatchDetailPerson & { characters?: KunLanguage[] })[]
}

interface PatchEntityIntro {
  lang: Language
  intro: string
  source?: string
  machine?: boolean
}

interface PatchEntityLink {
  name: string
  url: string
}

interface PatchCharacterTrait {
  id: number
  name: string
  group: string
  spoiler: number
  lie: boolean
}

interface PatchStaffRole {
  role_key: string
  role_name: string
  character?: string
}

// One work an entity is attached to. It IS a galgame card — the same one every
// list on the site draws — carrying the facts of the attachment beside it:
// roster_role / spoiler / voices for a character, roles for a credit name.
interface PatchEntityWork extends GalgameCard {
  roster_role?: string
  spoiler?: number
  voices?: PatchDetailPerson[]
  roles?: PatchStaffRole[]
}

// next_cursor is catalog's own opaque cursor, absent on the last page. It is
// never built here — only echoed back to /works.
interface PatchEntityWorkPage {
  works: PatchEntityWork[]
  next_cursor?: string
}

interface PatchCharacterDetail extends PatchEntityWorkPage {
  id: number
  name: KunLanguage
  aliases: string[]
  image_hash?: string
  figure_hash?: string
  gender?: number
  // A character birthday has no year: catalog publishes MM-DD.
  birth_m?: number
  birth_d?: number
  intros: PatchEntityIntro[]
  traits: PatchCharacterTrait[]
  links: PatchEntityLink[]
}

interface PatchStaffDetail extends PatchEntityWorkPage {
  id: number
  name: KunLanguage
  aliases: string[]
  photo_hash?: string
  gender?: number
  birth_y?: number
  birth_m?: number
  birth_d?: number
  siblings: PatchDetailPerson[]
  intros: PatchEntityIntro[]
  links: PatchEntityLink[]
}

interface PatchDetailRatingBucket {
  score: number
  count: number
}

// Each source keeps its own scale — vndb / bangumi 0-10, erogamescape 0-100,
// dlsite 0-5 — and catalog never normalizes between them. Divide by the
// per-source max in KUN_EXTERNAL_RATING_MAP before comparing two of them.
interface PatchDetailRating {
  source: string
  score: number
  vote_count: number
  rank?: number
  distribution?: PatchDetailRatingBucket[]
}

interface PatchDetailSeries {
  id: number
  name: string
  galgame_count: number
}

interface PatchDetail extends GalgameCard {
  introduction_markdown: KunLanguage
  introduction_html: KunLanguage
  updated: string
  tags: PatchDetailTag[]
  officials: PatchDetailOfficial[]
  characters: PatchDetailCharacter[]
  staff: PatchDetailStaffGroup[]
  ratings: PatchDetailRating[]
  series: PatchDetailSeries[]
  wiki_engine_ids: number[]
}
