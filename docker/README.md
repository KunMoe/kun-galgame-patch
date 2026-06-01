# Docker deployment — moyu (kun-galgame-patch-next)

moyu is a **downstream** patch site. Its containers (`api`, `web`) are
stateless; the backing services (Postgres / Redis) and upstreams (oauth /
galgame-wiki / image_service / MinIO) are **owned by the kun-galgame-infra hub**.
This setup mirrors the hub's `docker/` conventions one-for-one.

## Layout

| File | Builds | Base image | Why |
|---|---|---|---|
| `docker/go.Dockerfile` | `server` + every `cmd/migrate` / `sync-moemoepoint` / … tool (pure Go) | `distroless/static` (~15–25 MB) | `CGO_ENABLED=0` static binary. moyu has **no** cgo deps → no `cgo.Dockerfile` (unlike the hub's oauth/image). |
| `docker/nuxt.Dockerfile` | `web` (Nitro `node-server`) | `node:22-slim` (~390 MB) | self-contained `.output`; sharp ships via the `@kun/ui` layer build |

Both are **parametric** (`--build-arg CMD=…` / `APP=…`) and require the **repo
root** as build context (`apps/web` consumes `packages/ui` as a Nuxt layer from
source).

## Run

moyu does not own infra, so bring the **hub** up first (it creates the shared
network `kun-galgame-infra_default` + Postgres/Redis/oauth/galgame/image):

```bash
# 1) in kun-galgame-infra: start shared infra + upstreams (see its docker/README)
#    docker compose up -d postgres redis minio meili oauth image galgame
# 2) here:
cp docker/api.env.example docker/api.env && $EDITOR docker/api.env   # fill secrets
docker compose build
docker compose run --rm migrate     # moyu SQL migrations (after the DB exists)
docker compose up -d api web
```

| Service | URL |
|---|---|
| moyu API health | http://localhost:15010/api/v1/health |
| moyu web | http://localhost:15011 |

Host ports use the **1xxxx** range to coexist with a running `air` dev server.
Service-to-service traffic uses container ports via service names
(`postgres:5432`, `http://oauth:9277`, `http://galgame:9280`, `http://image:9278`).

### Prerequisite: the `kungalgame_patch` database

moyu shares the hub's Postgres. Add its database to the hub's
`docker/initdb.d/01-create-databases.sh` so it's created on first init:

```sql
CREATE DATABASE kungalgame_patch;
```

(The schema itself is built by `docker compose run --rm migrate` here, not initdb.)

## Configuration

- **Backend** (`docker/api.env`, 12-factor `env_file`): hosts are hub service
  names, not localhost. `KUN_SERVER_MODE=prod` makes config **fail fast** if
  `KUN_IMAGE_SERVICE_BASE_URL` / `KUN_IMAGE_CDN_BASE` are unset (audit GPT-L02).
  Rotate every `__SET_ME__` secret for a real deploy; prefer `docker secret`/a
  vault over `env_file`.
- **Frontend**: public config (`apiBase`, oauth*) is **baked at build** from the
  `PUBLIC_*` build args in `docker-compose.yml` (mapped to the
  `KUN_VISUAL_NOVEL_NUXT_PUBLIC_*` / `NUXT_PUBLIC_KUN_OAUTH_*` names nuxt.config
  reads). To build once and configure at runtime instead, set the canonical
  `NUXT_PUBLIC_*` env (see `docker/web.env.example`).

## Health checks

distroless ships no shell/wget, so the Go binary self-probes via a `healthcheck`
subcommand (`pkg/health`): the compose healthcheck runs `/app healthcheck`,
which GETs its own `/api/v1/health` and exits 0/1. The frontend uses a Node TCP
liveness probe.

## image_service — known gap (not fixed here)

The hub serves images at `KUN_IMAGE_PUBLIC_BASE_URL` with object key
`{aa}/{bb}/{hash}.webp` (no `/img` in the key). moyu's frontend still
**hardcodes** `imageBed = https://image.moyu.moe` in `app/config/moyu-moe.ts`
and **adds `/img/`** in `resolveAvatarUrl.ts` / `resolveBannerUrl.ts`. For
hash-addressed avatars/banners to resolve, set `KUN_IMAGE_CDN_BASE` (backend)
**and** that hardcoded `imageBed` so that `imageBed + /img` equals the hub's
`KUN_IMAGE_PUBLIC_BASE_URL`. The clean fix (make `imageBed` env-driven and drop
the hardcoded `/img` so moyu produces the same URL image_service returns) is a
pending frontend change — see the cross-repo audit notes.

## Gotchas (same as the hub)

- **No BuildKit/buildx** on this host → the Dockerfiles avoid
  `--mount=type=cache` (plain layer caching only).
- **sharp arch**: the Nuxt build bundles `sharp` for the build container's
  arch; build + run both happen in the same linux arch, so they match. Don't
  copy a host-built `.output` into the image.
- **Migrations** are a one-off job (profile `jobs`), never auto-run on boot.

## Three-repo orchestration

Put an umbrella `website/compose.yaml` one level up that `include:`s the hub +
kungal + moyu composes, and define `postgres`/`redis`/`minio`/`meili` **only in
the hub**. When included, drop the `external` network block at the bottom of
this file (all services share one project network). Front the lot with
Caddy/Traefik by domain.
