FROM golang:1.25-alpine AS builder
WORKDIR /app

# Install git if needed by go mod
RUN apk add --no-cache git

# Cache module downloads
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Build with cached module and build cache
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w" -o library-syncer ./cmd/main.go

FROM alpine:3.21 AS runner
RUN apk add --no-cache ffmpeg python3 py3-pip && \
    pip install --no-cache-dir --break-system-packages yt-dlp
WORKDIR /app
RUN mkdir -p /app/data/secrets
RUN mkdir -p /app/assets/covers
RUN mkdir -p /app/data/db
RUN mkdir -p /app/sheets
COPY --from=builder /app/assets/covers /app/assets/covers
COPY --from=builder /app/library-syncer .
CMD ["./library-syncer"]
