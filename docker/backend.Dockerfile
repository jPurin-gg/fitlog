# backend/Dockerfile
FROM golang:1.24.2

WORKDIR /app

EXPOSE 8080
# 実行は docker compose 側で: go run main.go
