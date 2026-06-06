#
# Parametric build for moyu's PURE-GO binaries: the HTTP server (cmd/server)
# and the one-off cmd/migrate / sync-moemoepoint / remap-patch-ids / etc. tools.
#
# moyu has NO cgo dependencies (pgx + disintegration/imaging are pure Go), so
# every binary is a CGO_ENABLED=0 static binary on distroless — there is no
# cgo.Dockerfile here (unlike the oauth/image services in the infra repo).
#
#   docker build -f docker/go.Dockerfile --build-arg CMD=server  -t moyu/api .
#   docker build -f docker/go.Dockerfile --build-arg CMD=migrate -t moyu/migrate .
#
# Build context MUST be the repo root.
ARG GO_VERSION=1.26

# ---- build ----
FROM golang:${GO_VERSION}-trixie AS build
WORKDIR /src
# Manifests first → module-download layer is cached until go.mod/sum change.
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
ARG CMD=server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/app ./cmd/${CMD}

# ---- run ----
# distroless/static: ~2MB base, no shell, nonroot. Bundles ca-certificates
# (outbound HTTPS: OAuth, Wiki, image_service, B2/S3, SMTP TLS) + tzdata.
# The server self-probes via `/app healthcheck` (pkg/health) for the container
# HEALTHCHECK, since distroless has no shell/wget. Ports live in compose.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/app /app
# cmd/migrate reads SQL from /migrations at runtime. Its -path defaults to the
# RELATIVE "migrations", so the runtime CWD matters — and distroless :nonroot
# sets WORKDIR=/home/nonroot, which would make the default resolve to a MISSING
# /home/nonroot/migrations → filepath.Glob returns empty (no error) → the tool
# silently applies ZERO migrations ("没有待执行的迁移"). `WORKDIR /` below fixes
# the default; compose also passes `-path /migrations` explicitly as a backstop.
COPY apps/api/migrations /migrations
# About-page content: the static .mdx posts that cmd/server reads at runtime
# (internal/about, cfg.About.PostsDir). They live in the WEB app's source tree
# (apps/web/posts) and are NOT DB data, so no migration step carries them —
# bake them into the api image so it is self-contained. Point the server at
# them with KUN_POSTS_DIR=/posts (docker/api.env). The banner images under
# apps/web/public/posts are served separately by the web container.
COPY apps/web/posts /posts
# Override distroless :nonroot's WORKDIR=/home/nonroot so cmd/migrate's default
# relative -path "migrations" resolves to the baked /migrations (see above).
WORKDIR /
USER nonroot:nonroot
ENTRYPOINT ["/app"]
