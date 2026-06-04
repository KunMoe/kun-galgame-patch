#
# Tools image: EVERY apps/api/cmd/* binary in one image, for moyu's one-off
# migration jobs the api/migrate images don't carry — e.g. migrate-oauth-prep,
# remap-patch-ids, backfill-release-date (see docs/migration + 03-bootstrap §B).
#
# The per-service Dockerfile builds ONE binary (ARG CMD); this bundles them all
# and invokes a job by name:
#
#   docker run --rm --network kun-galgame-infra_default \
#     --env-file docker/api.env ghcr.io/kun1007/moyu-tools remap-patch-ids
#
# moyu has no cgo → pure static binaries. Build context MUST be the repo root.
ARG GO_VERSION=1.26

# ---- build ----
FROM golang:${GO_VERSION}-trixie AS build
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
# -o <dir>/ writes one binary per cmd package, named after its directory.
RUN mkdir -p /out && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/ ./cmd/...

# ---- run (debian-slim; binaries on PATH, invoked by name) ----
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 10001 --create-home --shell /usr/sbin/nologin appuser
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
USER appuser
# No ENTRYPOINT: run a job by name, e.g. `docker run ... moyu-tools migrate`.
