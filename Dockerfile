FROM golang:1.25-alpine AS builder
WORKDIR /app

# cache deps
COPY go.mod go.sum ./
RUN go mod download

# build
COPY . .
RUN ./scripts/build.sh

FROM alpine:3.21 AS runner
RUN apk add --no-cache rsync openssh-client ca-certificates
WORKDIR /app
COPY --from=builder /app/library-syncer .
COPY --from=builder /app/data/covers ./data/covers
RUN mkdir -p /app/data/secrets
RUN mkdir -p /app/data/db
RUN touch /app/data/secrets/code.txt
CMD ["./library-syncer"]
