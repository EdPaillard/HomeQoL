# ── Stage 1: builder ──────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# gcc + musl-dev are required by go-sqlite3 (CGO).
# git is needed for go mod download with VCS-based deps.
RUN apk add --no-cache gcc musl-dev git ca-certificates

WORKDIR /app

# Copy dependency manifests first so Docker cache skips the download
# step when only source files change.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build. CGO_ENABLED=1 is mandatory for go-sqlite3.
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /gateway ./cmd/server

# ── Stage 2: minimal runtime image ────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates  → outbound HTTPS (Powens, Conso API, Jellyfin, TMDB…)
# tzdata           → time.LoadLocation("Europe/Paris") inside the binary
# wget             → used by the Docker healthcheck below
RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /gateway .

# Create the data directory before dropping privileges.
# SQLite needs to write here — appuser must own it.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup \
    && mkdir -p /app/data \
    && chown -R appuser:appgroup /app

USER appuser

# data/ is where SQLite writes — mounted as a named volume in compose
# so the DB survives container restarts and image rebuilds.
VOLUME ["/app/data"]

EXPOSE 8080

ENTRYPOINT ["./gateway"]