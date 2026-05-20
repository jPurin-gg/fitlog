# fitlog

## 本番デプロイ

ローカル開発では通常の env ファイルを使います。

```sh
cp docker/env/backend.env.example docker/env/backend.env
cp docker/env/frontend.env.example docker/env/frontend.env
cp docker/env/db.env.example docker/env/db.env
docker compose up -d
```

本番では `*.prod.env` と `.env.prod` を使います。これらは機密情報を含むため git には上げません。

```sh
# まだ作っていない場合だけ作成します。
cp .env.example .env.prod
cp docker/env/backend.env.example docker/env/backend.prod.env
cp docker/env/frontend.env.example docker/env/frontend.prod.env
cp docker/env/db.env.example docker/env/db.prod.env
```

特に次の値は本番環境に合わせて変更してください。

- `NEXT_PUBLIC_API_URL`: 公開されるバックエンドAPIのURL。別APIドメインを使うなら `https://api.example.com` のように設定します。空欄にすると、ブラウザは同一オリジンの `/api` を呼び、Next.js が Docker 内部のバックエンドへ中継します。
- `BACKEND_INTERNAL_URL`: Next.js が `/api` を中継する Docker 内部のバックエンドURL。通常は `http://backend:8080` のままでOKです。
- `FRONTEND_URL`: バックエンドのCORSで許可するフロントエンドURL。
- `DB_PASSWORD`: `backend.prod.env` と `db.prod.env` で同じ値にします。
- `OPENAI_API_KEY`: AIプロバイダーのAPIキー。

本番用コンテナを起動します。

```sh
docker compose -f compose.prod.yml --env-file .env.prod up -d --build
```

`compose.prod.yml` はデフォルトでフロントエンドとバックエンドを `127.0.0.1` にだけ公開します。外部公開はリバースプロキシや Cloudflare Tunnel 側で行う想定です。PostgreSQL はホストに公開しません。
