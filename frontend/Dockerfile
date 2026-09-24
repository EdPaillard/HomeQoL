# ── Stage 1: build ────────────────────────────────────────────────────────────
# Use the official Node LTS image. The linux/amd64 → linux/arm64 swap is
# handled automatically by Docker when building on the Oracle ARM instance —
# no changes needed here.
FROM node:20-alpine AS builder

WORKDIR /app

# Copy lockfile first so Docker cache skips npm install when only source changes.
# Supports both npm (package-lock.json) and yarn (yarn.lock) — whichever exists.
COPY package.json ./
COPY package-lock.json* yarn.lock* ./

RUN npm ci
# Copy source
COPY . .

# ── Build-time environment variables ──────────────────────────────────────────
# These are baked into the JS bundle at build time by Vite.
# Pass them with --build-arg when running docker build, or set them in
# docker-compose.yml under build.args.
#
# VITE_API_URL        — Go backend URL, e.g. http://oracle-ip:8080/api/v1
#                       If nginx proxies /api/ → gateway, use /api/v1
# VITE_JELLYFIN_API_KEY — from Jellyfin dashboard → API Keys
#                         Can be left empty if Jellyfin isn't set up yet;
#                         the MediaPage shows a friendly error instead.
ARG VITE_API_URL=/api/v1
ARG VITE_JELLYFIN_API_KEY=""

ENV VITE_API_URL=$VITE_API_URL
ENV VITE_JELLYFIN_API_KEY=$VITE_JELLYFIN_API_KEY

RUN npm run build

# ── Stage 2: serve ────────────────────────────────────────────────────────────
# Minimal nginx image — only the built static files land here, not Node or
# any build tooling. Final image is typically ~25 MB.
FROM nginx:alpine

# Remove the default nginx welcome page
RUN rm -f /usr/share/nginx/html/index.html

# Our nginx config handles:
#   - SPA fallback (all unknown paths → index.html)
#   - Reverse proxy for /api/, /jellyfin/, /transmission/, /grafana/ etc.
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy the Vite build output
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

# nginx writes logs to stdout/stderr so Docker captures them with
# `docker compose logs frontend`
CMD ["nginx", "-g", "daemon off;"]
