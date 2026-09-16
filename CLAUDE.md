# kun-galgame-patch (moyu) — AI Agent Project Guide

## 铁律 (Iron Rules — non-negotiable; these override every other guideline in this file)

1. **No background gradients in any UI, ever.** Never use gradient backgrounds in UI design (`bg-gradient-*`, `from-*/via-*/to-*`, `linear-gradient()`, `radial-gradient()`, `conic-gradient()`, etc.); use solid colors from the project's palette. **One sanctioned exception**, granted 2026-08-28 and carried by an inline comment at the site: the galgame card's bottom scrim (`components/galgame/Card.vue`), a black bottom-to-top fade that keeps the view/download counts legible over an arbitrary cover — the same one the forum card uses. Do not remove it in a no-gradient sweep, and do not read it as licence for a second one.
2. **Prefer KunUI components; do not modify KunUI itself.** When adding or changing frontend UI, reach for a KunUI component (`@kungal/ui-*`) first — do not hand-roll a native/custom component unless there is genuinely no KunUI equivalent for what you need. If KunUI appears to have a bug or is missing a feature, **do not edit KunUI's code** (it is a shared upstream library) — report it to the user directly instead, and let them decide how to proceed.
3. **The page id IS the catalog work id — `patch.id == catalog_work.id`, always, and no second copy of it exists.** These were two id spaces until 2026-09-13, and the divergence was an accident rather than a design: catalog minted a fresh identity when it imported the retired wiki instead of preserving the wiki id, and the offset then drifted. Measured over production's 64,694 kungal claims as `gid - work_id`: median **+185** across the 62,574 wiki-era rows, a constant **−163,863** across the 1,938 this site numbered itself after the wiki retired, and **0 / 0 / 0 across the 182 games minted since** — the design had already converged and only the legacy block was stranded. `cmd/align-patch-ids` closed it (migrations 037/038).
   - There is **no `patch.catalog_work_id`** and no mapping helper. Re-adding either re-creates the class of bug this ended: 63 rows carried another game's work id, derived once by a rule that was wrong for `wiki-<n>` rows, and nothing ever refreshed them.
   - A page catalog cannot name lives at `patch.id >= model.LocalOnlyIDBase` (1,500,000,000), never on a small integer — catalog will eventually mint a work with that number and the collision is silent.
   - `claimed_by.work_id` / `product_work_id` is the **FORUM's** page id, not ours. Both downstreams answer to catalog site `kungal` and kungal still carries the legacy gid; it surfaces as `forum_gid` and is the only thing a link to kungal may use.
   - **The old numbers did not vanish.** 9,519 pre-037 page ids are also a live catalog work id, so `/patch/<n>` and `/galgame/<n>` name different games for 8,484 pages (5,731 published). `/patch/<n>` resolves ONLY out of `patch_redirect` with `scope='legacy'` and 404s on a miss; `/galgame/<n>` reads only `scope='merge'`. One number can legitimately hold **both** rows with different targets — 24 do — which is why that table is keyed `(scope, old_id)` since migration 039, never `old_id` alone. Never infer one namespace from the other: substituting id spaces does not error, it renders a different game — kungal's collection page did exactly that for 192,178 of production's 252,544 folder items.


galgame **patch / resource site**. `apps/api` = Go Fiber v3 + GORM + Postgres, `apps/web` = Nuxt 4.
This repo is one of the **downstreams of nextmoe-infra (OAuth / identity / contract hub)** (the other being kun-galgame-forum / kungal).

## Core Engineering Principles

> Shared baseline across all KUN Galgame repositories. Defaults, not dogma — apply judgment.

1. All commit messages must be written entirely in English.
2. All code comments must be written entirely in English.
3. Keep each source file under ~500 lines where practical; once a file grows past ~300 lines, consider splitting it (a guideline, not a hard rule).
4. Write every frontend function as an arrow function; compose/merge class names with `cn` wherever practical.
5. Deliberately balance elegant modularity against necessary duplication — choose per case instead of always favoring either.
6. Constantly verify that frontend and backend agree on the data: field shapes and response formats must match what each side expects.
7. After every change, watch for unintended side effects elsewhere.
8. If a change requires running a migration, tell the user explicitly at the end — which command, and against which database.
9. Always seek the most modern, elegant solution that fits the project's current state; consult the latest official docs and resources online when useful.
10. Never let the pursuit of elegance or modularity make the code complex or hard to follow, and don't write over-defensive code.
11. A Nuxt page — and any component used as a page/route root — must have a **single real root element**: never `display: contents` (generates no box, so the transition can't attach) and never a leading comment / whitespace / sibling at the template root (a comment is itself a root node). Either trips Nuxt's "does not have a single root node" warning and drops the page-transition enter animation (the page appears without animating). Keep explanatory comments *inside* the root element.
12. Reserve the scrollbar gutter globally — `html { scrollbar-gutter: stable }`, with an `overflow-y: scroll` `@supports` fallback — so the document width is constant across routes. Otherwise navigating from a scrolling page to a height-locked one (no scrollbar) removes the classic scrollbar's ~15px and the centered layout shifts sideways: a "teleport" at the tail of the page transition. This is a browser layout fact, not a transition bug. Use single-edge `stable` (`both-edges` is buggy in Chrome); it's a harmless no-op under overlay scrollbars (macOS/iOS).
13. **One task = one Codex session; one assigned target repo = one branch = one worktree.** Never let two sessions write this checkout; prefer `codex-session new kun-galgame-patch <session>`. The launcher exposes source-repo reference material through `$CODEX_SESSION_REFS` when present; read it in place and never copy it into any worktree. A single-repo session may write only its own worktree; an explicitly coordinated cross-repo operation may write only separately assigned target worktrees. Launcher source checkouts and refs are always read-only.
14. DB-backed tests must use only the launcher-provided, explicit, unique `TEST_DATABASE_DSN`; never discover or fall back to a DSN from `.env`, and never print the DSN. Run shared-database Go integration suites with `-count=1 -p 1`, never against a live or rehearsal database.

## Comments

**Default: none.** Code that can be understood by reading it gets no comment. Most code is that code. `[review]`

**A comment is earned by a mistake that already happened, not by one you predict.** Do not comment while writing — you cannot tell yet which parts are traps. Comment when something went wrong there: an agent or a person got it wrong, a review caught it, a test went red, production broke. The comment records the wrong conclusion that was actually reached, so the next reader does not reach it again. If you cannot name the incident, there is no comment to write. `[review]`

Two standing exceptions, where the comment is a record rather than a warning:

- `apps/api/migrations/**` — a migration is history and cannot be re-read from the current schema. Say what it changes and why, including what was done about existing rows.
- A constraint that is true but invisible from this file: a version floor, an upstream bug, a required ordering. `huma/v2 >= v2.39.0` is one; a reader who does not know it will "simplify" the dependency back and break SSE.

Cross-service identity and ownership boundaries count as invisible constraints when their authority lives in infra. Keep the shortest comment at the exact seam where confusing the identities or sources would fail silently.

This policy governs source code. Concise onboarding or incident notes in configuration files such as `.env.example`, `.air.toml`, and Compose files remain allowed.

Write the conclusion, not the mechanism. `// splitCommand takes the subcommand off before flag.Parse` is a restatement; `flag.Parse stops at the first non-flag argument, so 'migrate down -steps 1' parsed no flags and rolled back nothing` is the trap. Quote real system output verbatim when reproducing a symptom.

Never write: restatements of the code, section banners, `TODO` without an owner, or doc comments that only echo the identifier (`// New creates a new X`). Exported Go identifiers get a doc comment only when the name alone is ambiguous. If a comment explains what a name means, rename the thing and delete the comment.

English, and short. When in doubt, delete it — a wrong comment costs more than a missing one, and the missing one gets written the day it is needed.

## Current catalog cutover state

- The `w161-p4` line moves publish/claim/withdraw and the cron inbox to catalog. Read the read-only source-workspace file at `${CODEX_SESSION_REPO%/*}/nextmoe-infra/refs/proj/161-n5-grand-window.md`—not a `../` path from this worktree—and verify the branch, migration `029_claim_event_processed`, client binding, and deployment state before assuming the window ran.
- `/galgame/messages/feed` is retired. The staged cron consumes `GET /v2/catalog/claim-events` with a separate processed-event table and cursor namespace; do not revive the wiki feed or reuse its idempotency keys.
- The display axis is **mirrored, not swept**. `GET /v2/catalog/changes` (spec 2.1.0) is a declared mirror channel: every write that moves a work's claim state, its display axis (`display_nsfw` / `content_rating`) or its existence bumps `updated_at` and surfaces the id there, oldest first, and an empty cursor enumerates the whole population so the first drain *is* the full inventory — there is no id-listing face to sweep and no nightly full scan to add back. moyu caches the verdict in `patch.content_limit` (migration 032) purely as a SQL pre-filter applied to the builder that produces both the page and its `COUNT`; `NULL` means not-yet-mirrored and **passes**, and catalog's own gate at hydrate time stays the authority, which is why a stale value costs a short row and never an NSFW leak. Hydrate with **both gates open** (`nsfw=true`, no `content_limit`) or the works this is meant to mark nsfw are exactly the ones the request hides. Key the result on the id the feed hands over — it is the patch id (铁律 3). This used to read the claim's `site_work_id` or the `curated` ref and never the catalog id, because filing a work under its catalog id then marked a different game. The cursor is an opaque `cur_…` string in `cron_state.last_cursor` — a separate namespace from the claim feed's integer watermark. `gone: true` is consumed as a skip today; a merged-away id also appears on `GET /v2/catalog/redirects`, which is what `patch_redirect` with `scope='merge'` exists to record.
- Catalog `site` and catalog source key are different identities. Preserve the Wave 161 dual-read/fallback rules for legacy anchors and registry-issued ids.
- 方案③ (2026-08-21, letmoe + infra signed; kungal/moyu reuse): catalog is the existence layer. `/galgame` browse and site search do **not** send `claim_state`. Users do not claim games; the write that indexes a page is publishing a resource. `patch.published` is the sticky SEO flag (first resource, not cleared on delete; migration 031). Hidden/ban still unpublishes. Do not reintroduce a user-facing 认领 flow or a local full replica.
- The two galgame lists are one endpoint: `/galgame` is moyu's patch resource list and `/galgame?library=true` is the catalog information library, which the site mirrors as `/galgame` and `/gallib` (kungal draws the same line). `indexed=true` is the sitemap's own lane and stays local. Taxonomy detail pages belong to the library side and list the whole catalog, never only the games that have resources here.
- Catalog **reads and catalog edit** go through public API v2 (`pkg/catalogv2`): `KUN_NEXTMOE_API_BASE` is the origin (no `/v1` suffix); send `Authorization: Bearer nmk_…` (problem+json, string ids, cursor). User edits hit `/v2/me/proposals` and `/v2/moderation/snapshots`; claim writes `POST /v2/me/claims` and `PATCH {state: live|pending|withdrawn}` with a required `If-Match` (`*` matches). 撤回 also deletes: `DELETE /v2/me/claims/{id}` soft-deletes the **catalog work**, not just the claim, so it runs only on a claim read as `pending` and under that read's ETag — a live claim's work predates this site and deleting it takes a VNDB entry down with the patch page. Declined claims can be neither withdrawn nor deleted (catalog allows withdraw only from live|pending, delete only from draft); the 撤回 button 409s on them. The merged-proposal count reads `GET /v2/catalog/proposals`. `nm_test_` / `nm_live_` keys 401 on `/v2`. **Every v2 read must name what it wants**: a detail face answers a bare id+name row for each block the request does not list in `include=`, and `spoiler=` defaults to `none` — both render an empty page instead of erroring, which is how the first cutover silently dropped 会社 logos, 声优, rating histograms, spoilered tags and the whole character/staff modal. The token lists live beside the calls in `pkg/catalogv2`; an unknown token is a 400, so they track catalog's `apiv2/collect` specs. Do not reintroduce the v1 envelope client, `X-API-Key` on `/v2`, or any `/api/v1/**` catalog path: infra deleted the v1 surface (410 tombstones) and `pkg/catalogclient` is gone. Two v2 faces carry constraints the code depends on. `POST /v2/me/claims` mints from `field_values` alone (wave R4) — no `work_id` (422 beside it) and no top-level `display_name`, because the map's `catalog.work.display_name` is only a seed that `applyTitles` rewrites to the `olang` official title. Every claim read pins `site=kungal` (`catalogv2.SiteKungal`, moyu's `oauth_clients.catalog_site`): unpinned, the feed and the merged-proposal tally answer every tenant, whose `product_work_id` names another site's rows. `GET /v2/catalog/claim-events` additionally needs the `claim_events:read` scope on the app key — operator-granted, not self-service — and pages at `limit<=100` with an opaque `cur_` cursor the cron mints from its stored watermark.

## Cross-Service Contracts (Inviolable — owned by nextmoe-infra)

The active authoritative contract docs are synced as **read-only mirrors** under `docs/{oauth,image_service,artifact}/` (files carry a GENERATED banner in the header). Catalog is consumed from infra/portal without a vendored mirror; galgame-wiki is a retired portal/tombstone contract, not a live local mirror.
**To change a contract, go change it at the infra source — do not touch the copies here**; the copies are regenerated by kungal-docs' `pnpm docs:sync`. Core invariants:

- **Identity (C1/C2)**: `user.id` is the **same integer** across this database, OAuth, and the other downstream — never renumber users; local tables use `*_user_id` to align with OAuth `users.id`. OAuth owns identity and issues JWTs; this service only **verifies signatures and issues no tokens** (see `internal/middleware/auth.go`).
- **User profile (C6)**: **do not persist it to the local user table and do not treat it as the source of truth** (a short-TTL in-memory cache is fine — `pkg/userclient` already has a built-in ~10min TTL); fetch by id list via `GET /users/batch` (OAuth Client Basic Auth, ≤100 ids, **does not return** email / moemoepoint / created_at). For @mention completion use `GET /users/search` (**do not cache**); for the current user use `/oauth/userinfo`. OAuth ships no SDK — implement a thin client yourself.
- **moemoepoint (C3)**: a single balance per user, **single source in OAuth**; if a local column for it exists, it is only a cached view. Granting/deducting goes through the s2s API, idempotency key = `<app>:<event>:<ref>` (e.g. `moyu:wiki_approved:1207`). Reasons available to downstreams: `content_approved` / `content_removed` / `daily_checkin` / `liked`; **reserved by OAuth, forbidden over s2s**: `admin_grant` / `admin_deduct` / `migration` / `register_gift`. The s2s endpoints are **already implemented** (`POST/GET /users/:id/moemoepoint`, `Adjust` is idempotent; see infra `internal/platform/auth/handler/moemoepoint_handler.go` and `cmd/oauth/main.go`).
- **Images (C4)**: the content-addressed image store lives in OAuth; **moyu does not run its own S3**, and both avatars and images use the OAuth image store. URL = `{base}/{aa}/{bb}/{hash}[_variant].webp` (two-level hex sharding); pass `*_image_hash` fields and resolve them with the image client.
- **Catalog claims (C5 successor)**: catalog owns claim lifecycle events. Synchronization consumes `GET /v2/catalog/claim-events`; never add a new consumer of the retired wiki message feed.

For active vendored contracts see `docs/oauth/`, `docs/image_service/`, and `docs/artifact/`; for catalog and the galgame retirement tombstone use the infra-owned source/portal.

## Comment walls live in the community primitive, not in this database

Every comment on this site is a post in NextMoe's community primitive
(`kun_community`), read and written over `KUN_COMMUNITY_API_BASE` by
`pkg/communityclient`. `patch_comment` and `user_patch_comment_like_relation`
are FROZEN — the import's source and the rollback site — and nothing reads them.
moyu-side notes and the trade-offs are in
`docs/proj/community-comments-cutover.md`; the contract, the importer and the
deployment order live in infra's `docs/community/`.

- **The community tenant is `moyu`, and it is NOT `catalog_site`.** It comes from
  `oauth_clients.community_site` (empty falls back to `catalog_site`, which is
  how the forum, letmoe and sticker keep working). moyu's `catalog_site` stays
  `kungal` because the catalog claim link needs it — reading either column for
  the other is what put this site's walls in the forum's tenant until
  2026-09-16, where one account purge blanked a user's FORUM comments. Anchors
  are therefore this site's own bare ids: `<patch.id>` (kind 1) and
  `<resource.id>` (kind 2), no prefix. A game wall's anchor is also the catalog
  work id (铁律 3), which is what lets infra's `cmd/retire-merged-comments` sweep
  a wall a merge stranded.
- `internal/community/anchor` is still the only place that decides what is
  moyu's, and an anchor it does not recognise is dropped from a feed rather than
  linked. That is not dead code under one tenant: `GET /posts` and
  `GET /search/posts` answer this site's threads **plus** every catalog-anchored
  one, which are a network-wide conversation by design.
- A comment id in a URL is a **community post id**. `#post-<id>` is the anchor;
  `#comment-<n>` is a pre-cutover comment id and resolves ONLY through
  `patch_comment_community_map` (`GET /patch/comment/locate?legacy_id=`). The two
  id spaces overlap, which is why the shapes differ.
- **Never read a wall with `POST /comments/resolve`** — it is get-or-create, and
  because three sites called it to render a page it had minted 110,918 empty
  threads upstream by 2026-09-15. Read `GET /comments` (no thread until the first
  comment), write `POST /comments`.
- **No local like mirror.** Every face that returns a post carries
  `reaction_count`, and `viewer_reacted` for the named viewer (`viewer_id` on a
  read, the acting `author_id` on `PATCH /posts/{id}`). The edit face answered 0
  until infra cce5b4a8, so this build needs a community deployed past it.
- The unread `total` counts the same rows as the unread list, including other
  sites' catalog-anchored threads the list drops, so the red dot can exceed the
  list. Unreachable while production has no catalog-anchored thread (0 on
  2026-09-16).
- `patch.comment_count` is a display counter only the write path can keep true;
  no SQL can recompute it. `merge.Fold` no longer moves comments and logs the
  stranded wall instead. Community-side moderation (a review-queue reject) drifts
  it, the same class of drift favourites already accept.
- Pre-moderation is gone (the primitive has none; `site_setting` never carried an
  enabled `comment_verify`), reporting is the primitive's weighted flag +
  review queue, and `enforce.Registry` keeps only `patch_resource`.
- The reference-ping cron cannot see comment bodies any more. It sweeps
  `GET /posts` for `/image/<hash>` tokens instead, and a failed sweep must fail
  the whole run: half a sweep leaves the other half of the images unreferenced
  and collectable.

## This Repo's Key Points

- **Minimal post-migration auth**: no local login / 2FA, **issues no tokens** (identity belongs entirely to OAuth, this service only verifies signatures). The session itself is a **BFF opaque session** (`moyu_session` cookie + Redis storing the OAuth token, see `internal/middleware/auth.go`), with **90-day sliding renewal** (active users no longer get logged out every week) — for the model and the 2026-06 fix see `docs/proj/session-lifetime.md`.
- `docs/{oauth,image_service,artifact}/` are infra mirrors (including `image_service/03-api-design.md` — its early implementation corrections have been folded back into the infra source, and as of 2026-06 it is no longer a variant). To change them, change them in infra and then `docs:sync`; **do not touch the copies here**.
- **Database schema changes must come with a migration reminder**: this repo's schema goes through `apps/api/migrations/NNN_*.up.sql` (idempotent, `IF NOT EXISTS`) plus a bundled migrate runner, and **does not run AutoMigrate on startup**. Whenever you add a migration file / change a table structure, **at the end of the task you must explicitly tell the user: whether a migration needs to be run in production, and which command to run**. Skipping it → live code reads a column that does not exist → silent failure (cf. the 2026-06 infra moemoepoint granting incident: a single missing column caused the whole site to be unable to receive moemoepoint for ~29h).
