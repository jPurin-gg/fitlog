# backend/Dockerfile
FROM golang:1.24.2

WORKDIR /app

# Install air for hot reloading
RUN go install github.com/air-verse/air@latest

EXPOSE 8080
# 実行は docker compose 側で: air または go run main.go
