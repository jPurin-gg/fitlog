# fitlog

## 本番デプロイ

サンプルから環境変数ファイルを作成します。

```sh
cp .env.example .env
cp docker/env/backend.env.example docker/env/backend.env
cp docker/env/frontend.env.example docker/env/frontend.env
cp docker/env/db.env.example docker/env/db.env
```

特に次の値は本番環境に合わせて変更してください。

- `NEXT_PUBLIC_API_URL`: 公開されるバックエンドAPIのURL。フロントエンドのビルド時に埋め込まれます。
- `FRONTEND_URL`: バックエンドのCORSで許可するフロントエンドURL。
- `DB_PASSWORD`: `backend.env` と `db.env` で同じ値にします。
- `OPENAI_API_KEY`: AIプロバイダーのAPIキー。

本番用コンテナを起動します。

```sh
docker compose -f compose.prod.yml --env-file .env up -d --build
```

`compose.prod.yml` はデフォルトでフロントエンドとバックエンドを `127.0.0.1` にだけ公開します。外部公開はリバースプロキシや Cloudflare Tunnel 側で行う想定です。PostgreSQL はホストに公開しません。
