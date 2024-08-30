# syntax=docker/dockerfile:1

# ========== BUILD ==========
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN go install github.com/air-verse/air@v1.52.3

COPY go.mod go.sum ./

RUN go mod download

COPY . .

EXPOSE 5001

# ENTRYPOINT [ "sh", "-c", "chmod +x /app/tmp && air" ]

CMD ["air"]
