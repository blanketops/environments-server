# ── Build stage ───────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for private module fetching (if needed)
RUN apk add --no-cache git

# Copy module files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o blanketops-server .

# ── Runtime stage ─────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/blanketops-server .

# Copy any static assets if you serve them from Go
# COPY --from=builder /app/static ./static

# Use non-root user (distroless default is nonroot:65532)
USER nonroot:nonroot

# Expose the server port
EXPOSE 8082

ENTRYPOINT ["./blanketops-server"]