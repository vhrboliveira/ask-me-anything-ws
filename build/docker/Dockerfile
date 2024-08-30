# syntax=docker/dockerfile:1

# ========== BUILD ==========
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o ama-go ./cmd

# ========== PROD ==========
# minimalist docker image
FROM scratch

WORKDIR /app

COPY --from=builder /app/ama-go .

EXPOSE 5001

CMD ["./ama-go"]