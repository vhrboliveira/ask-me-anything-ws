# syntax=docker/dockerfile:1

# ========== BUILD ==========
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache curl bash


WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

RUN go install github.com/jackc/tern/v2@latest

COPY . .

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o ama-go ./cmd/ama
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o tern-go ./cmd/tools/terndotenv

# ========== PROD ==========
FROM gcr.io/distroless/base

COPY --from=busybox /bin/sh /bin/sh

WORKDIR /app

COPY --from=builder /app/ama-go .

COPY --from=builder /app/tern-go .

COPY --from=builder /go/bin/tern /usr/local/bin/tern

COPY --from=builder /usr/local/bin/entrypoint.sh /usr/local/bin/entrypoint.sh

COPY --from=builder /app/internal/store/pgstore/migrations /app/internal/store/pgstore/migrations

EXPOSE 5001

ENTRYPOINT ["/bin/sh", "/usr/local/bin/entrypoint.sh"]