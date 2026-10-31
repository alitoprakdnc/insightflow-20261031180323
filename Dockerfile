# syntax=docker/dockerfile:1

# ---------- Go build ----------
FROM golang:1.22-alpine AS go-builder

# Install git (required for some go modules) and ca‑certificates for HTTPS fetches
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache module download – copy only go.mod/go.sum first
COPY src/backend/go.mod src/backend/go.sum ./
RUN go mod download

# Copy the rest of the backend source code
COPY src/backend/ .

# Build the Go binary (statically linked, stripped)
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o /app/backend ./...

# ---------- Frontend build ----------
FROM node:20-alpine AS ui-builder

WORKDIR /ui

# Install only production dependencies first (caching layer)
COPY src/frontend/package*.json ./
RUN npm ci --prefer-offline --no-audit --progress=false

# Copy the rest of the frontend source code
COPY src/frontend/ .

# Build the React application (expects a `build` output folder)
RUN npm run build

# ---------- Runtime ----------
FROM alpine:3.19 AS runtime

# Minimal runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the compiled Go binary
COPY --from=go-builder /app/backend ./backend

# Copy the compiled static assets (served by the Go server)
COPY --from=ui-builder /ui/build ./static

# Production‑ready environment defaults
ENV GIN_MODE=release
ENV TZ=UTC

# Expose the port the Go API listens on (adjust if your service uses a different port)
EXPOSE 8080

# Optional health‑check (assumes a /health endpoint exists)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s \
  CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./backend"]