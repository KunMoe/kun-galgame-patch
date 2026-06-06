#
# Build for moyu's Nuxt 4 frontend (Nitro node-server preset), apps/web.
#
# Build context MUST be the repo root: apps/web `extends: ['@kun/ui']`, a Nuxt
# LAYER consumed from source, so packages/ui must be in the context.
#
# Public runtime config (apiBase, oauth*) is read by nuxt.config.ts from custom
# KUN_*/NUXT_PUBLIC_KUN_* env names at BUILD time, so it is passed as build args
# and baked. Any public key can still be overridden at RUNTIME via the canonical
# NUXT_PUBLIC_* names (NUXT_PUBLIC_API_BASE, NUXT_PUBLIC_OAUTH_SERVER_URL,
# NUXT_PUBLIC_OAUTH_WEB_URL, NUXT_PUBLIC_OAUTH_CLIENT_ID,
# NUXT_PUBLIC_OAUTH_REDIRECT_URI) — see docker/README.md.
#
#   docker build -f docker/nuxt.Dockerfile --build-arg APP=web -t moyu/web .
#
# NOTE: the image CDN (`imageBed`) is still HARDCODED in app/config/moyu-moe.ts
# (not env-driven) — align it with image_service before relying on hash-
# addressed avatar/banner display. See docker/README.md §image_service.
ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-trixie-slim AS base
RUN corepack enable
WORKDIR /repo

# ---- deps: copy every workspace manifest, install only the target subgraph ----
FROM base AS deps
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/web/package.json    apps/web/package.json
COPY apps/api/package.json    apps/api/package.json
COPY packages/ui/package.json packages/ui/package.json
ARG APP=web
# --ignore-scripts: the app's `postinstall: nuxt prepare` can't run yet (source
# isn't copied); the later `nuxt build` runs prepare itself.
RUN pnpm install --frozen-lockfile --ignore-scripts --filter "@apps/${APP}..."

# ---- build ----
FROM deps AS build
ARG APP=web
# Frontend public config, baked at build. Empty args fall back to the in-config
# defaults (`process.env.X || '<default>'`).
ARG PUBLIC_API_BASE=
ARG PUBLIC_OAUTH_SERVER_URL=
ARG PUBLIC_OAUTH_WEB_URL=
ARG PUBLIC_OAUTH_CLIENT_ID=
ARG PUBLIC_OAUTH_REDIRECT_URI=
ARG PUBLIC_UMAMI_ID=
ENV KUN_VISUAL_NOVEL_NUXT_PUBLIC_API_BASE=${PUBLIC_API_BASE} \
    NUXT_PUBLIC_KUN_OAUTH_SERVER_URL=${PUBLIC_OAUTH_SERVER_URL} \
    NUXT_PUBLIC_KUN_OAUTH_WEB_URL=${PUBLIC_OAUTH_WEB_URL} \
    NUXT_PUBLIC_KUN_OAUTH_CLIENT_ID=${PUBLIC_OAUTH_CLIENT_ID} \
    NUXT_PUBLIC_KUN_OAUTH_REDIRECT_URI=${PUBLIC_OAUTH_REDIRECT_URI} \
    KUN_VISUAL_NOVEL_FORUM_UMAMI_ID=${PUBLIC_UMAMI_ID}
COPY packages/ui packages/ui
COPY apps/${APP} apps/${APP}
# The @kun/ui Nuxt layer needs its own .nuxt generated (its `prepare` was
# skipped by --ignore-scripts, and .dockerignore strips the host's copy); the
# app build reads the layer's generated tsconfig.
RUN pnpm --filter @kun/ui run prepare
RUN pnpm --filter "@apps/${APP}" run build
# build:limit bumps Node's heap (--max-old-space-size=8192). The web build is
# memory-heavy and OOM-aborts (exit 134 / SIGABRT) under the default heap in
# CI's constrained build env; the GitHub runner has 16 GB so 8 GB heap fits.
RUN pnpm --filter web run build:limit

# ---- run: just Node + the self-contained .output (no pnpm, no sources) ----
# sharp ships inside .output (built for linux in this same arch container).
FROM node:${NODE_VERSION}-trixie-slim AS run
ARG APP=web
ENV NODE_ENV=production HOST=0.0.0.0 NITRO_PORT=3000
WORKDIR /app
COPY --from=build /repo/apps/${APP}/.output ./.output
# Home carousel: server/api/home/carousel.get.ts reads pinned .mdx from
# process.cwd()/posts (= /app/posts). Those static posts live in the source
# tree (apps/web/posts) and are NOT bundled into .output, so copy them in or
# the carousel returns []. (The /about page reads the same .mdx via the Go API,
# which bakes its own copy — see api go.Dockerfile.)
COPY --from=build /repo/apps/${APP}/posts ./posts
USER node
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
