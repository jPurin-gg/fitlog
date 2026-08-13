# ビルド
FROM golang:1.24.2 AS builder
WORKDIR /src
COPY ./backend/go.mod ./backend/go.sum ./
RUN go mod download
COPY ./backend ./
RUN CGO_ENABLED=0 go build -o /out/app .

# 実行
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/app /app/app
COPY --from=builder /src/prompts /app/prompts
COPY --from=builder /src/tmpkin_jp.json /app/tmpkin_jp.json
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/app"]
