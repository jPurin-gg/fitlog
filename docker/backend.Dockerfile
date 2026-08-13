# backend/Dockerfile
FROM golang:1.24.2

WORKDIR /app

# Pin the development tool so rebuilding the image cannot silently change it.
RUN go install github.com/air-verse/air@v1.62.0

EXPOSE 8080
# 実行は docker compose 側で: air または go run main.go
