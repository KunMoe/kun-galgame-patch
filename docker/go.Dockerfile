#
# Parametric build for moyu's PURE-GO binaries: the HTTP server (cmd/server)
# and the one-off cmd/migrate / sync-moemoepoint / remap-patch-ids / etc. tools.
#
# moyu has NO cgo dependencies (pgx + disintegration/imaging are pure Go), so
# every binary is a CGO_ENABLED=0 static binary on distroless — there is no
# cgo.Dockerfile here (unlike the oauth/image services in the hub repo).
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
USER nonroot:nonroot
ENTRYPOINT ["/app"]
