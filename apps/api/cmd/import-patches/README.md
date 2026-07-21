# cmd/import-patches

Bulk-import standardized Chinese-patch archives into moyu, replacing the old
standalone `sync-patch` tool. Per file: parse the legacy filename → map VNDB id
to a Wiki `galgame_id` (identities are shared: `patch.id == galgame_id`) → ensure
the local patch carrier → upload the bytes through the **artifact service**
(init → PUT straight to B2 from disk → complete; **no blake3**) → insert an
artifact-backed `patch_resource`.

Internal archive job: every row is owned by `--user-id` (default **2310**) and it
**skips the moemoepoint award + favorited-user notifications** that
`PatchService.CreateResource` performs. Idempotent — re-runs skip files already
imported for a galgame (dedup on `(galgame_id, name)`, `name` = sanitized filename).

## Flags

| flag | default | meaning |
|------|---------|---------|
| `--dir` | `./patch` | directory of `.rar/.zip/.7z` archives |
| `--delete-list` | "" | path to an increment's `delete_list.txt`; its superseded files' archive-owned resources are removed **before** the import phase (mirrors `delete_old.py`) |
| `--dry-run` | false | probe only: parse + wiki-check + dedup, **no upload/write/delete** |
| `--user-id` | 2310 | archive account that owns imported patches/resources |
| `--vndb` | "" | comma-separated VNDB ids to restrict imports to, e.g. `v14,v36` |
| `--limit` | 0 | process at most N recognized files (0 = all; for testing) |

Deletes are guarded to `user_id == --user-id` rows only — a user-uploaded
resource is never touched. `art.Delete` soft-deletes the artifact blob, the row
is hard-deleted (FK cascades take likes / favorites / history), and aggregates
are recomputed.

## Wiki drafts (status=2) — post-run remediation

`CheckGalgameByVndbID` returns `exists=true` even for **unclaimed VNDB drafts**
(wiki `status=2`, auto-created by `sync-vndb`). The importer happily creates
patches on them, but wiki's public `/galgame/batch` + `/galgame/:id` return
**status=0 only**, so those galgames — and their imported resources — are
**invisible on moyu** (homepage / list / detail) until published. Claiming needs
a user/admin JWT the S2S importer can't obtain, so the run instead **detects and
reports** the drafts at the end with the exact fix:

```
UPDATE galgame SET status=0 WHERE id IN (<ids>) AND status=2;   -- on kun_galgame_wiki
reindex-search --index=galgames                                 -- rebuild Meilisearch
```

Run both after any import that logs `UNPUBLISHED wiki drafts`. (`status 2→0` via
raw SQL skips the search write-through hook, hence the reindex.)

Config is read from the environment (`godotenv.Load()` + `config.Load()`), same
keys as `moyu-api`: `KUN_DATABASE_URL`, `KUN_ARTIFACT_SERVICE_BASE_URL`,
`KUN_ARTIFACT_OAUTH_CLIENT_ID/_SECRET` (fall back to `OAUTH_CLIENT_ID/_SECRET`),
`KUN_NEXTMOE_API_BASE`, `KUN_SERVER_MODE`.

## Build (static, for scp to prod)

```bash
cd apps/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/import-patches ./cmd/import-patches
scp /tmp/import-patches kungal-neo:/srv/import/
```

## Run on kungal-neo (dokploy)

moyu + infra run as dokploy containers on the **`dokploy-network`** overlay; the
internal service DNS names (`artifact:9279`, `catalog:9281`, `postgres:5432`) only
resolve **on that network**, and artifact's presigned PUTs go to **Backblaze B2
over HTTPS** (egress + CA certs needed). So run the importer as a one-off
container on `dokploy-network`, reusing the `moyu-api` image (has certs) with the
static binary + patch files bind-mounted, and moyu's own env:

```bash
# 1. materialize moyu-api's env (contains secrets — keep 600, delete after)
sudo docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' \
  kun-visual-novel-patch-ndxhtp-moyu-api-1 > /srv/import/moyu.env
chmod 600 /srv/import/moyu.env

# 2. dry-run probe (read-only: parse + wiki + dedup)
sudo docker run --rm --network dokploy-network --env-file /srv/import/moyu.env \
  -v /srv/import/import-patches:/import-patches:ro \
  -v /srv/import/patches:/patches:ro \
  --entrypoint /import-patches ghcr.io/kunmoe/moyu-api:latest \
  --dir /patches --dry-run

# 3. sample import (1–2 known-on-wiki ids), verify, then full run
sudo docker run --rm --network dokploy-network --env-file /srv/import/moyu.env \
  -v /srv/import/import-patches:/import-patches:ro \
  -v /srv/import/patches:/patches:ro \
  --entrypoint /import-patches ghcr.io/kunmoe/moyu-api:latest \
  --dir /patches --vndb v14
```

## Getting the files (torrent → individual patch archives)

Each batch (增量5, 增量6) is delivered as a torrent of **spanned-RAR segments**
(`增量.partNN.rar`, ~26 GB each). Extracting the outer archive yields the
individual per-game patch `.rar` files (flat) that `--dir` consumes, plus a
`删除旧文件/delete_list.txt` used by `--delete-list`.

kungal-neo has **no BitTorrent client or RAR extractor installed**, and we do
**not** install on the host — run them in throwaway containers that install the
tools inside themselves (`aria2` + `unar`, both in Debian main; `unar` handles
multi-volume / RAR5 without the non-free `unrar`). ~60 GB free needed per batch
(parts + extracted); host has ~115 GB free.

```bash
# fetch the .torrent (GitHub, public)
curl -fsSL -o /srv/import/6.torrent \
  'https://raw.githubusercontent.com/GalGame-Work/vn_patch/main/VN%E5%90%88%E9%9B%86%E5%BD%92%E6%A1%A3%E5%A2%9E%E9%87%8F6/VN%E5%90%88%E9%9B%86%E5%BD%92%E6%A1%A3%E5%A2%9E%E9%87%8F6.torrent'

# download the segments (throwaway container, host untouched)
docker run --rm -v /srv/import:/w -w /w debian:stable-slim sh -c \
  'apt-get update -q && apt-get install -y -q aria2 && aria2c --seed-time=0 -d dl 6.torrent'

# extract outer spanned archive -> individual patch .rar files
docker run --rm -v /srv/import:/w -w /w debian:stable-slim sh -c \
  'apt-get update -q && apt-get install -y -q unar && unar -q -o patches dl/*/增量.part01.rar'

# then: locate 删除旧文件/delete_list.txt from the extracted/torrent content and pass it
#   import-patches --delete-list /srv/import/.../delete_list.txt --dir /srv/import/patches
```
