# Fitlog

Fitlogは、筋力トレーニングの計画と記録を管理するWebアプリです。月間プランの作成、当日のセット記録、カレンダーでの振り返りに加え、xAIのGrok APIを使ったメニュー作成やコーチングを利用できます。

Next.js、Go、PostgreSQLで構成され、開発環境はDocker Composeで起動します。

## 公開環境

<https://fitlog.purin.blog>

このインスタンスは、自宅サーバー上にセルフホストして運用しています。

## 主な機能

- ニックネームとパスワードによるログイン・ユーザー登録
- トレーニング環境や使用器具などのコーチ設定
- AIによる月間トレーニングプランの生成
- 月間プランを基にした当日のメニュー作成（AI障害時は基礎プランで開始）
- 月間プランがない場合のフリーワークアウト
- 重量、回数、感触、自己ベストのセット記録
- 保存したセットを基にした次セットのAI提案
- ワークアウト終了時の即時集計と、分離されたAIコメント
- 種目の検索、追加、お気に入り、最近使用した種目の表示
- AIによる代替種目の提案
- カレンダーからの過去記録・未来予定の確認と編集

## 技術構成

| 領域 | 使用技術 |
|---|---|
| フロントエンド | Next.js 15、React 19、TypeScript、Tailwind CSS 4 |
| バックエンド | Go 1.24、`net/http` |
| データベース | PostgreSQL 16 |
| AI | xAI Grok Chat Completions API |
| 開発環境 | Docker Compose |

ブラウザは通常、フロントエンドと同一オリジンの `/api` を呼びます。Next.jsがそのリクエストをDockerネットワーク内のGoバックエンドへ中継します。

## ディレクトリ構成

```text
.
├── backend/
│   ├── main.go                         # 設定、DB、HTTPサーバーの起動
│   ├── seed.go                         # 種目seedデータの埋め込み
│   ├── tmpkin_jp.json                  # 組み込み種目カタログ
│   ├── internal/
│   │   ├── app/                        # 依存の組み立てとルーティング
│   │   ├── auth/                       # 認証と署名Cookie
│   │   ├── exercise/                   # 種目、お気に入り、種目設定
│   │   ├── planning/                   # 月間・日次プラン
│   │   ├── workout/                    # セット記録、提案、集計
│   │   ├── profile/                    # AIコーチ設定
│   │   ├── reporting/                  # ダッシュボードとカレンダー
│   │   ├── ai/openaicompat/            # OpenAI互換APIアダプター
│   │   ├── database/                   # DB接続とマイグレーション
│   │   ├── httpx/                      # JSON、エラー、ミドルウェア
│   │   └── prompt/                     # プロンプトテンプレートの描画
│   └── prompts/                        # AIプロンプト
├── frontend/
│   ├── app/                            # Next.js App Routerの画面
│   ├── components/                     # 共通UI
│   └── lib/                            # APIクライアントと共通処理
├── docker/                             # Dockerfileと環境変数の例
├── compose.yaml                        # 開発環境
├── compose.prod.yml                    # 本番向け構成
├── compose.test.yml                    # 並行性を確認する一時PostgreSQL環境
└── docs.md                             # API・バックエンド技術仕様
```

バックエンドは機能単位のモジュラーモノリスです。各機能の中心パッケージがユースケースとインターフェースを定義し、`httpapi` と `postgres` がそれぞれHTTP・PostgreSQLアダプターとして中心パッケージに依存します。具体実装の組み立ては主に `internal/app` が担当します。

## ローカル開発

### 必要なもの

- Docker
- Docker Compose
- Node.js 22.14以上（または24以上）とnpm（ブラウザE2Eをホスト上で実行する場合）
- AI機能を使う場合は、xAI APIキー

### 1. 環境変数ファイルを作成する

```sh
cp docker/env/backend.env.example docker/env/backend.env
cp docker/env/frontend.env.example docker/env/frontend.env
cp docker/env/db.env.example docker/env/db.env
```

作成したファイルはGit管理対象外です。

### 2. DB設定をそろえる

`docker/env/backend.env` と `docker/env/db.env` に、同じデータベース接続情報を設定します。

| `backend.env` | `db.env` |
|---|---|
| `DB_USER` | `POSTGRES_USER` |
| `DB_PASSWORD` | `POSTGRES_PASSWORD` |
| `DB_NAME` | `POSTGRES_DB` |

`DB_HOST=db` と `DB_PORT=5432` は、Docker Composeで起動する場合は変更不要です。

### 3. バックエンド設定を確認する

`docker/env/backend.env` で、少なくとも次を確認してください。

- `SESSION_SECRET`: 32 bytes以上のランダムな値に変更する
- `XAI_API_KEY`: xAI Consoleで発行したAPIキー
- `XAI_API_URL`: xAI Chat Completionsエンドポイント（既定 `https://api.x.ai/v1/chat/completions`）
- `XAI_MODEL`: 利用するGrokモデル（既定 `grok-4.3`）
- `AI_OPTIONAL_TIMEOUT_SECONDS`: 日次調整・次セット提案・AI総評を待つ上限秒数（既定15秒）

`SESSION_SECRET` は、例えば次のコマンドで生成できます。

```sh
openssl rand -base64 48
```

APIキーが未設定でもサーバーは起動します。月間プラン生成や代替種目提案は利用できませんが、日次メニューは基礎プランで開始でき、セット記録と終了集計はAIと無関係に保存できます。

### 4. 起動する

```sh
docker compose up -d --build
```

起動後のURL:

- Webアプリ: <http://localhost:3000>
- バックエンドAPI: <http://localhost:8080>
- PostgreSQL: `localhost:5432`

ログを確認する場合:

```sh
docker compose logs -f frontend backend
```

停止する場合:

```sh
docker compose stop
```

`db_data` ボリュームにデータが保存されます。ボリュームを削除する操作は、保存済みデータの消失につながります。

## 認証とAPI

- 初回ログイン時、未登録のニックネームは新しいユーザーとして登録されます。
- ログイン成功時にHMAC-SHA256署名済みの `HttpOnly` Cookieを発行します。
- 保護APIのユーザーはCookieから確定し、リクエストで `user_id` は受け付けません。
- APIの成功レスポンスはリソースJSONを直接返します。
- エラーは `application/problem+json` 形式で返します。
- セット保存には `Idempotency-Key` ヘッダーが必要です。
- セットはDB保存完了を画面に返した後、別APIでAI提案を取得します。AIの待機中や失敗時でも、同じ重量・回数で次へ進めます。
- ワークアウト終了APIはDB集計を即時に返し、AI総評は専用APIで後から取得・再試行します。
- 当日開始レスポンスの `ai_status` は `applied`、`fallback`、`not_requested` のいずれかです。

エンドポイントやリクエスト形式の詳細は [docs.md](./docs.md) を参照してください。

## DBマイグレーション

スキーマの正本は `backend/internal/database/migrations/*.sql` です。

バックエンドの起動時に次の処理を行います。

1. PostgreSQLへ接続する
2. アドバイザリロックを取得する
3. 未適用のマイグレーションをファイル単位のトランザクションで適用する
4. 適用したファイル名を `schema_migrations` に記録する
5. 種目テーブルが空の場合、組み込みの種目カタログを投入する

マイグレーションとseedはバックエンドが管理します。DockerのPostgreSQLコンテナはデータベースを起動・永続化するだけで、独自の初期化SQLは実行しません。新規DBと既存DBのどちらも、バックエンド起動時の同じマイグレーション経路で更新されます。

種目カタログはGoバイナリに埋め込まれているため、実行環境でseedファイルのパスを設定する必要はありません。既存の種目が1件以上ある場合、seed処理は既存データを変更せずスキップします。

## 検証

コンテナの起動後、次のコマンドでバックエンドとフロントエンドを検証できます。

```sh
docker compose exec -T backend go test ./...
docker compose exec -T backend go test -race ./...
docker compose exec -T backend go vet ./...
docker compose exec -T frontend npm run lint
docker compose exec -T frontend npx tsc --noEmit
docker compose exec -T frontend npm test
docker compose exec -T frontend npm run build
docker compose exec -T frontend npm audit --omit=dev
```

PostgreSQLの行ロックと冪等性を実DBで確認する統合テストは、開発DBと分離した一時DBで実行します。

```sh
docker compose -f compose.test.yml run --rm --build backend-test
docker compose -f compose.test.yml down
```

ブラウザE2EはバックエンドAPIをモックし、「保存成功 → AI失敗 → 継続 → 終了集計 → AI総評再試行」を確認します。初回だけChromiumを準備してください。既にインストール済みのChromeを使う場合は `PLAYWRIGHT_CHANNEL=chrome` を指定できます。

```sh
cd frontend
npm ci
npx playwright install chromium
npm run test:e2e
# または: PLAYWRIGHT_CHANNEL=chrome npm run test:e2e
```

## 本番環境

本番用ファイルのひな形を作成します。

```sh
cp .env.example .env.prod
cp docker/env/backend.env.example docker/env/backend.prod.env
cp docker/env/frontend.env.example docker/env/frontend.prod.env
cp docker/env/db.env.example docker/env/db.prod.env
```

起動前に、コピーしたすべてのファイルを本番環境に合わせて編集してください。特に次の項目は、そのまま使用しないでください。

- `.env.prod` の `NEXT_PUBLIC_API_URL`
- `backend.prod.env` の `SESSION_SECRET`
- `backend.prod.env` と `db.prod.env` のDBユーザー・パスワード・DB名
- `backend.prod.env` の `XAI_API_KEY`、`XAI_API_URL`、`XAI_MODEL`
- `backend.prod.env` の `FRONTEND_URL`

同一オリジンでNext.jsからバックエンドへ中継する場合、`NEXT_PUBLIC_API_URL` は空欄にします。APIを別オリジンで公開する場合は、ブラウザから到達可能なAPI URLを設定してください。

HTTPSで運用する場合は、`SESSION_COOKIE_SECURE=true` に設定します。

設定後に起動します。

```sh
docker compose -f compose.prod.yml --env-file .env.prod up -d --build
```

本番Composeは、デフォルトではフロントエンドとバックエンドを `127.0.0.1` にのみ公開し、PostgreSQLはホストへ公開しません。外部公開にはリバースプロキシやトンネルを前段に配置してください。
