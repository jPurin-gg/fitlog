# Fitlog

筋トレの月間計画、当日のセット記録、AIコーチングをまとめるローカルファーストなWebアプリです。Next.js、Go、PostgreSQLをDocker Composeで起動します。

## ローカル起動

```sh
cp docker/env/backend.env.example docker/env/backend.env
cp docker/env/frontend.env.example docker/env/frontend.env
cp docker/env/db.env.example docker/env/db.env
docker compose up -d --build
```

起動前に `docker/env/backend.env` の次の値を設定してください。

- `SESSION_SECRET`: 32 bytes以上のランダム値。例: `openssl rand -base64 48`
- `OPENAI_API_KEY`: 使用するOpenAI互換AIプロバイダーのAPIキー
- DB設定: `docker/env/db.env` と同じユーザー、パスワード、DB名

ブラウザは <http://localhost:3000>、バックエンドAPIは <http://localhost:8080> です。通常、ブラウザは同一オリジンの `/api` を呼び、Next.jsがバックエンドへ中継します。

## 構成

バックエンドはモジュラーモノリスです。機能ごとに `auth`、`exercise`、`planning`、`workout`、`profile`、`reporting` を分け、各機能内でユースケースとHTTP/PostgreSQLアダプターを分離しています。

```text
backend/
  main.go                    # composition root / server lifecycle
  internal/
    app/                     # route wiring
    auth/                    # signed-cookie authentication
    exercise/                # exercise catalog, favorites, settings
    planning/                # monthly and daily plans
    workout/                 # set records, recommendations, summaries
    profile/                 # AI coaching preferences
    reporting/               # dashboard and calendar read models
    ai/openaicompat/         # single OpenAI-compatible adapter
    database/                # connection and embedded migrations
    httpx/                   # JSON, Problem Details, middleware
    prompt/                  # prompt template renderer
```

依存方向は原則として `HTTP/DB adapter -> feature service -> feature interface` です。`main.go` と `internal/app` だけが具体実装を組み立てます。

## APIと認証

- ログイン成功時にHMAC署名済みの `HttpOnly` Cookieを発行します。
- 保護APIのユーザーはCookieから確定し、リクエストの `user_id` は受け付けません。
- 成功時はリソースJSONを直接返します。
- 失敗時は `application/problem+json` を返します。
- セット保存とAI提案は別APIです。セット保存には `Idempotency-Key` が必須です。

API一覧は [docs.md](./docs.md) を参照してください。

## DBマイグレーション

スキーマの正本は `backend/internal/database/migrations/*.sql` です。バックエンド起動時に未適用分だけトランザクション内で適用し、`schema_migrations` に記録します。複数プロセスの同時起動はPostgreSQLのアドバイザリロックで直列化します。

`docker/db/init.sql` はアプリケーションテーブルを作りません。既存の永続ボリュームも、バックエンド起動時に同じマイグレーション経路で更新されます。

## 検証

```sh
docker compose exec -T backend go test ./...
docker compose exec -T backend go test -race ./...
docker compose exec -T backend go vet ./...
docker compose exec -T frontend npm run lint
docker compose exec -T frontend npx tsc --noEmit
docker compose exec -T frontend npm run build
docker compose exec -T frontend npm audit
```

## 本番デプロイ

```sh
cp .env.example .env.prod
cp docker/env/backend.env.example docker/env/backend.prod.env
cp docker/env/frontend.env.example docker/env/frontend.prod.env
cp docker/env/db.env.example docker/env/db.prod.env
docker compose -f compose.prod.yml --env-file .env.prod up -d --build
```

本番では少なくとも次を変更してください。

- `SESSION_SECRET`: 開発環境と別のランダム値
- `SESSION_COOKIE_SECURE=true`: HTTPS運用時に必須
- `FRONTEND_URL`: 実際のフロントエンドOrigin
- `DB_PASSWORD`: バックエンドとPostgreSQLで同じ強い値
- `OPENAI_API_KEY` / `OPENAI_API_URL` / `OPENAI_MODEL`: 採用するプロバイダー設定
- `NEXT_PUBLIC_API_URL`: 別API Originを公開する場合のみ指定。同一Origin中継なら空欄

本番Composeはデフォルトでフロントエンドとバックエンドを `127.0.0.1` にだけ公開します。外部公開はリバースプロキシまたはトンネルを前段に置く想定です。
