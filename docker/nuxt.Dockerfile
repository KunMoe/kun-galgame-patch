#
# Build for moyu's Nuxt 4 frontend (Nitro node-server preset), apps/web.
#
# Build context MUST be the repo root (the install needs the workspace
# manifests). KunUI is consumed from npm (@kungal/ui-nuxt), not from source.
#
# `nuxt build` runs with NODE_ENV=production, so the image's runtimeConfig
# defaults are www.moyu.moe's (nuxt.config.ts `$production`). Any key can still
# be overridden at container start by its canonical NUXT_* name.
#
#   docker build -f docker/nuxt.Dockerfile --build-arg APP=web -t moyu/web .
ARG NODE_VERSION=24.22.1

FROM node:${NODE_VERSION}-trixie-slim AS base
RUN corepack enable
WORKDIR /repo

# ---- deps: copy every workspace manifest, install only the target subgraph ----
FROM base AS deps
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/web/package.json    apps/web/package.json
COPY apps/api/package.json    apps/api/package.json
ARG APP=web
# --ignore-scripts: the app's `postinstall: nuxt prepare` can't run yet (source
# isn't copied); the later `nuxt build` runs prepare itself.
RUN pnpm install --frozen-lockfile --ignore-scripts --filter "@apps/${APP}..."

# ---- build ----
FROM deps AS build
ARG APP=web
COPY apps/${APP} apps/${APP}
# build:limit bumps Node's heap (--max-old-space-size=8192). The web build is
# memory-heavy (jsdom is pulled in for server-side HTML sanitization) and
# OOM-aborts (exit 134 / SIGABRT) under the default ~4 GB heap; the GitHub
# runner has 16 GB so 8 GB heap fits.
RUN pnpm --filter "@apps/${APP}" run build:limit

# ---- run: just Node + the self-contained .output (no pnpm, no sources) ----
# sharp ships inside .output (built for linux in this same arch container).
FROM node:${NODE_VERSION}-trixie-slim AS run
ARG APP=web
ENV NODE_ENV=production HOST=0.0.0.0 NITRO_PORT=3000
WORKDIR /app
COPY --from=build /repo/apps/${APP}/.output ./.output
USER node
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
