# gid ≡ work_id — the id realignment and its cutover

Written 2026-09-13, before the window. Every number here was measured on
production (`kungalgame_patch` and `kun_catalog` on the infra postgres) that day.

## Why

This site's page id and the catalog work id were two numbers for one game. The
split was an accident, not a design: catalog minted a fresh identity when it
imported the retired wiki (62,583 rows, ids up to 62,810) instead of preserving
the wiki id, and the offset then drifted as rows were skipped and other sources
interleaved. After the wiki retired, the two sequences simply ran in parallel.

Measured over 64,694 kungal claims, as `gid - work_id`:

| gid range | rows | min / median / max | equal |
|---|---|---|---|
| 1 – 62,810 (the wiki import) | 62,574 | −185,615 / **+185** / +285 | 1.5% |
| 62,811 – 70,000 (own sequence) | 1,938 | / **−163,863** / | 0.4% |
| 200,000+ (games minted now) | 182 | **0 / 0 / 0** | **100%** |

The design had already converged — a game created today takes catalog's id as
its page id — and only the legacy block was stranded. Every derived copy of the
mapping rotted against it: 63 patch rows carried **another game's** work id,
because `resolveCatalogWork` read the `n` in a `wiki-<n>` vndb placeholder as a
catalog work id when it is this site's own gid, which catalog stores as a
`curated` ref. Nothing ever refreshed those rows — `SetCatalogWorkID` had zero
callers.

## What the renumber does

`cmd/align-patch-ids` asks catalog which work each page stands for, using the
rule the site itself routed by (`curated` ref, then the gid as a catalog id when
that work does not claim another page), and renumbers the table to match.

```
patch rows            10,957
already identical      1,031
renumbered             9,923   (9,896 realigned + 27 parked)
folded (duplicates)        3 groups, 3 pages merged away, 6 join rows dropped
parked (catalog cannot name it)  27   → 1,500,000,000 + old id
patch_redirect rows   10,957   (every pre-037 id, moved or not)
```

The three folds are pages that turn out to be one work, which is what catalog's
own merges leave behind. The survivor is the page a reader should keep landing
on (published, then resources, then a real page over a stub, then the older id),
and in all three cases that agrees with catalog's claim:

```
work   1241   keep 1245,  fold 5235
work   4082   keep 4115,  fold 4107   ← 1 resource + 62 downloads met 2 + 90
work 214969   keep 62560, fold 62992
```

## Why the URL moved to /galgame/:id

Because `work_id ≈ gid − 185`, **9,519 of the old page ids are also a live
catalog work id**. 8,484 renumbered pages are in that overlap and 5,731 of them
are published and indexed. On the old path the number would keep resolving and
simply mean a different game — a redirect cannot fix that, because the number is
legitimately taken.

So the two namespaces were separated:

- `/galgame/<id>` — canonical, `<id>` is the catalog work id. Reads
  `patch_redirect` only for `scope='merge'` (a work catalog merged away stops
  being a work, so there is no ambiguity).
- `/patch/<n>` — legacy only. Resolves **exclusively** out of `patch_redirect`
  with `scope='legacy'` and 404s on a miss. It never guesses.

That is also why the ledger carries a row for the 1,031 pages that did **not**
move: if a miss meant "same number", one absent row would quietly serve a
different game.

The three tabs became one page with `KunTabPanels` at the same time, so
`/patch/<n>/resource` and `/patch/<n>/comment` 301 to `/galgame/<id>?tab=…`.
121,222 notification links are rewritten in the same transaction.

## Cutover order

The window exists between step 2 and step 3: production is running the old
binary against renumbered data. Keep it short.

1. **Deploy the current image with migration 037 only.** It adds
   `patch_redirect` and flips the five child foreign keys to `ON UPDATE
   CASCADE`. Both are inert for the running code.
2. **Dry-run, read the plan, then apply.** From a host that can reach the prod
   database and catalog:
   ```
   go run ./cmd/align-patch-ids                 # writes align-patch-ids-<ts>.tsv
   go run ./cmd/align-patch-ids -apply          # one transaction
   ```
   Compare the printed counts against the table above before applying. The
   command refuses to run at all if 037 has not landed.
3. **Deploy the identity image with migration 038**, which drops
   `patch.catalog_work_id` and pushes the id sequence out of reach.

**Stop the API for step 2.** The first production attempt ran against live
traffic and died 57 seconds into the park hop with `ERROR: deadlock detected
(SQLSTATE 40P01)`, rolling the whole run back -- a page view increments
`patch.view` and reaches the same rows from the other side, and the renumber
holds them for minutes. With `moyu-api` stopped the same run took 3m44s end to
end. `applyPlan` now also takes every table it touches up front, which turns a
lost run into a stalled one, but that is a backstop and not a substitute.

Running them in any other order fails loudly rather than quietly: 038 before the
renumber leaves the favourites face with no mapping at all, and the renumber
before 037 aborts on `update or delete on table patch violates foreign key
constraint`.

## Rehearsed, 2026-09-13

The whole sequence ran against a full-size local copy (10,928 patch rows,
229,722 catalog works) before the window: migration 037, `-apply`, migration
038, then the site. The plan came out the same shape as production's and
**the three folds were identical** — same target works, same survivors, same
losers — which is the part that could not be checked any other way.

```
patch 行            10,928      已经恒等 1,026   需要改号 9,899
catalog 认不出          27       写入 patch_redirect 10,928
合并 -> work 1241 保留 1245 并入 5235 | 4082 保留 4115 并入 4107 | 214969 保留 62560 并入 62992
丢弃 6 条重复关系行（1 贡献 + 5 收藏），无用户文字
站内通知链接已重写 120,943
```

After it: no orphaned rows in any of the five child tables, every ledger
target is a live page, and no `/patch/` link left in `user_message`. The
overlap behaves as designed — old page 923 now answers at `/galgame/922`,
while `/galgame/923` is a different game entirely.

One thing the rehearsal changed: the resolve pass hit 960 transient timeouts
and the command used to abort on the first one, throwing the run away. It now
retries. Expect the resolve to take tens of minutes and to log retries; that
is normal, an abort is not.

## Ran in production, 2026-09-13

Window 23:15:08-23:18:52 +08 with `moyu-api` stopped; snapshot at
`/var/moyu-align/pre-align-20260913-230828.dump` on the host.

```
10,957 rows -> 1,031 identical / 9,896 realigned / 27 parked / 3 folded away
folds: work 1241 keep 1245 drop 5235 | 4082 keep 4115 drop 4107 | 214969 keep 62560 drop 62992
dropped 6 duplicate join rows (1 contribute + 5 favourite), no user-written text
10,954 patch rows, 10,957 ledger rows, id sequence at 2,000,000,000
```

The plan matched the rehearsal exactly on the folds, which was the part nothing
else could check.

Two numbers in the table above had drifted from an earlier dry run and are now
the applied ones: the ledger covers every pre-037 id (10,957, not 9,926), and 27
pages have no catalog work rather than 20. Of those 27, 26 are stubs; the one
real page is 7668 (`v134628`, one resource), which already had no
`catalog_work_id` before this ran -- catalog holds `curated:7668` against an
entity of another type, not a work. All 27 keep a ledger row, so their old URLs
still 301.

Afterwards: no orphaned rows in any of the five child tables, every ledger
target a live page, and no `/patch/` link left in `user_message`.

## Rollback

The renumber is one transaction, so a failure mid-flight leaves nothing behind.
After it commits there is no automatic reverse: `patch_redirect` holds
old → new for every row, so a reverse mapping is derivable, but the three folds
destroyed nothing recoverable only because the rows they dropped were join rows
(`user_patch_contribute_relation`, `user_patch_favorite_relation`,
`patch_link`) with no user-written text. Take a database snapshot before step 2.

## Still owed by infra and kungal

This closes moyu's half. The gid is **shared** — kungal's `galgame.id` is the
same integer and catalog's claim `product_work_id` names it — so two things are
outstanding and are asks, not work this repo can do:

1. **kungal renumbers the same way.** Until it does, its page ids stay in the
   legacy space and moyu links to it through `forum_gid` (the claim's
   `site_work_id`). After it does, `forum_gid == id` and that indirection can go.
2. **infra rewrites `catalog_work.product_work_id = catalog_work.id`** for the
   64,512 legacy kungal claims, at the same time as (1). Until then the claim
   keeps naming kungal's number, which is exactly what `forum_gid` reads.

Neither blocks this change: moyu no longer reads `product_work_id` as its own id
anywhere, and the claim-event cron now keys on `work_id`.
