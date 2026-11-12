# Fitlog

Fitlogは、筋力トレーニングの計画と記録を管理するWebアプリです。月間プランの作成、当日のセット記録、カレンダーでの振り返りに加え、xAIのGrok APIを使ったメニュー作成やコーチングを利用できます。

Next.js、Go、PostgreSQLで構成され、開発環境はDocker Composeで起動します。

## 公開環境

<https://fitlog.purin.blog>

このインスタンスは、自宅サーバー上にセルフホストして運用しています。

## 主な機能

- ニックネームとパスワードによるログイン・ユーザー登録
- トレーニング環境や使用器具などのコーチ設定
- ホーム・記録画面から切り替えられるAI利用トグル（初期OFF）
- AIによる月間トレーニングプランの生成
- 月間プランを基にした当日のメニュー作成（AI OFF・AI障害時は基礎プランで開始）
- 月間プランなしで最初から選べる手動のフリーワークアウト
- 重量、回数、感触、自己ベストのセット記録
- 保存したセットを基にした次セットのAI提案
- ワークアウト終了時の即時集計と、分離されたAIコメント
- 種目の検索、追加、お気に入り、最近使用した種目の表示
- AIによる代替種目の提案
- カレンダーからの過去記録・未来予定の確認と編集

## AIを使わずに記録する

AI利用は初期OFFです。開始画面の「自分で種目を選んで始める」から1種目選ぶと、月間プランを作らずに記録を開始できます。種目変更、重量・回数の保存、終了集計、履歴確認までAIなしで利用できます。保存済みの日次予定がある場合は、その予定とワークアウトを再利用します。

「AIを使う」はホーム、開始前、記録中、終了後、月間プランと代替種目のモーダルで切り替えられます。同じログインユーザー・ブラウザで設定を保持します。OFFにすると新しいAI呼び出しを止め、待機中の結果を破棄します。入力と保存済みの記録は維持し、AIによる開始調整の待機中ならAIなしで開始をやり直します。ONへ戻しても過去の記録は再送せず、終了後の総評は「AI総評を取得」から取得できます。

月間プランは任意です。当月のプランが未作成なら、その月に初めてホームを開いたときに案内します。AIがOFFでも案内は表示しますが、表示だけでAIを呼び出すことはありません。

- 「今月は次回以降表示しない」: 案内を閉じ、同じユーザー・ブラウザでは当月の再訪・再読み込みでも自動表示しません。翌月は再び案内します。
- 「閉じる」: 今回だけ閉じます。次のホーム訪問では再表示の対象になります。
- 「月間プランを作成／見直す」: 非表示にした月でも、ホームから手動で開けます。非表示設定はそのままです。
- 「プランなしで記録を始める」: ホームと案内から記録画面へ進めます。

作成済みの月やワークアウト記録中には自動案内を出しません。AI設定と当月の非表示設定はユーザー別のブラウザストレージに保存し、端末間では同期しません。ストレージが使えない場合、再読み込み後はAI OFFとなり、未作成の月の案内が再び表示されます。

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
│   ├── app/                            # Next.js App Routerの薄いルート入口
│   ├── features/                       # 機能別の画面、状態、API、型、専用UI
│   ├── components/                     # アプリ全体で共有するUI
│   ├── hooks/                          # アプリ全体で共有するReact hooks
│   └── lib/                            # HTTPクライアントと汎用処理
├── docker/                             # Dockerfileと環境変数の例
├── compose.yaml                        # 開発環境
├── compose.prod.yml                    # 本番向け構成
├── compose.test.yml                    # 並行性を確認する一時PostgreSQL環境
└── docs.md                             # API・バックエンド技術仕様
```

バックエンドは機能単位のモジュラーモノリスです。各機能の中心パッケージがユースケースとインターフェースを定義し、`httpapi` と `postgres` がそれぞれHTTP・PostgreSQLアダプターとして中心パッケージに依存します。具体実装の組み立ては主に `internal/app` が担当します。

フロントエンドも `dashboard`、`workout`、`calendar`、`exercises` の機能単位で整理しています。`app` の各 `page.tsx` はルーティングと機能画面の組み立てだけを担当し、機能固有の画面実装・API契約・型・UIは `features` に置きます。認証や下部ナビゲーションなど、複数機能から使うUIだけを共通 `components` に置きます。

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

APIキーが未設定でもサーバーは起動します。AI OFFのまま手動で種目を選べば、月間プランなしで開始・記録・終了できます。既存の月間プランがある場合は、AIを呼ばず基礎メニューで開始することもできます。

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
- セットはDB保存完了を画面に返した後、AI ONの場合だけ別APIで提案を取得します。AIの待機中や失敗時でも、同じ重量・回数で次へ進めます。
- ワークアウト終了APIはDB集計を即時に返し、AI ONの場合だけ専用APIで総評を取得・再試行します。
- 開始APIには `use_ai` を送信します。手動開始では `exercise_id` も指定でき、月間プランの参照やAI調整を行いません。
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

PostgreSQLの行ロックと冪等性、実HTTP経由の手動開始・記録・終了・履歴取得を確認する統合テストは、開発DBと分離した一時DBで実行します。月間プランなしでの記録、同時開始の重複防止、遅れたAI結果による上書き防止も対象です。AI通信先はテスト用のローカルサーバーに固定し、実際の外部AIは使いません。

```sh
docker compose -f compose.test.yml run --rm --build backend-test
docker compose -f compose.test.yml down
```

Goをホストで実行する場合は、`docker compose -f compose.test.yml up -d --wait db` の後、`backend` で `go test -race -tags=integration -count=1 ./internal/app ./internal/workout/postgres` を実行できます。既定では `127.0.0.1:55432` の検証DBを使い、テストごとに作成したスキーマだけを後片付けします。

ブラウザE2EはバックエンドAPIをモックし、AI OFFでの手動開始・複数セット・種目変更・終了、AI ONでの障害時の継続、月間案内の非表示・再訪・手動表示を確認します。初回だけChromiumを準備してください。既にインストール済みのChromeを使う場合は `PLAYWRIGHT_CHANNEL=chrome` を指定できます。

```sh
cd frontend
npm ci
npx playwright install chromium
npm run test:e2e
# または: PLAYWRIGHT_CHANNEL=chrome npm run test:e2e
# 起動ポートと並列数を指定する例:
# E2E_PORT=3002 PLAYWRIGHT_CHANNEL=chrome npm run test:e2e -- --workers=1
```

## 本番環境

新規セットアップでは、リポジトリのルートで `.env.example` を `.env` にコピーし、コメントに従って必須項目を埋めます。実キー・実パスワードはデプロイ先だけに保存し、Gitへコミットしないでください。

```sh
cp .env.example .env
chmod 600 .env
```

`.env` はComposeの変数展開に使われますが、全項目が自動的にコンテナへ渡るわけではありません。現行の `compose.prod.yml` は以下の3つの `env_file` も必要とします。バックエンド用は `.env` へのシンボリックリンクにし、AIキーの実値はデプロイ先の `.env` にだけ置きます。フロントエンド用ファイルにバックエンドの秘密情報をコピーしないでください。

```sh
ln -s ../../.env docker/env/backend.prod.env
cp docker/env/frontend.env.example docker/env/frontend.prod.env
cp docker/env/db.env.example docker/env/db.prod.env
chmod 600 docker/env/db.prod.env
```

既存の設定ファイルがある場合は、上の作成コマンドで上書きせず、既存設定とテンプレートを照合して不足分だけを反映してください。すでに実ファイルの `backend.prod.env` を運用している場合、以下のバックエンド設定はそのファイルへ反映します。`.env` への一本化は既存の秘密情報を保全したうえで行ってください。

起動前に次の値を設定してください（`change-me` はすべて置換）。変数の用途と既定値の全一覧は [.env.example](./.env.example) にあります。

| 設定先 | 項目 |
|---|---|
| `.env`（バックエンドにも渡る） | `SESSION_SECRET` を32 bytes以上のランダム値へ変更（生成例: `openssl rand -base64 48`）、`APP_ENV=production`、`FRONTEND_URL` を実際の公開オリジンへ変更、HTTPSで `SESSION_COOKIE_SECURE=true` |
| `.env` と `docker/env/db.prod.env` | `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` をそろえ、`.env` の `DB_USER` / `DB_PASSWORD` / `DB_NAME` にも対応する同じ値を設定 |
| `.env` | AIを使う場合のみ `XAI_API_KEY` を設定。`XAI_API_URL` / `XAI_MODEL` / `AI_*` 調整値を確認。キーなしでもAI OFFで記録可能 |
| `.env` と `docker/env/frontend.prod.env` | `NEXT_PUBLIC_API_URL` と `BACKEND_INTERNAL_URL` をそろえる（ビルド時と実行時の不一致を避ける） |

同一オリジンでNext.jsからバックエンドへ中継する場合、`NEXT_PUBLIC_API_URL` は空欄、`BACKEND_INTERNAL_URL` は `http://backend:8080` にします。APIを別オリジンで公開する場合は、ブラウザから到達可能なAPI URLを設定し、バックエンドの `FRONTEND_URL` をフロントエンドの公開オリジンに合わせます。これらのURLはフロントエンドのビルドにも使われるため、変更後は再ビルドが必要です。`NEXT_PUBLIC_*` には秘密情報を入れないでください。

`POSTGRES_*` は新規DBの初期化用です。既存の `db_data` がある場合、環境変数の変更だけでは既存DBの認証情報は更新されません。

`.env.example` の「今後の多Provider対応（#1〜#7）用」は [issue #1](https://github.com/jPurin-gg/fitlog/issues/1) の設定契約を記載した予約項目です。現在はxAIだけが実装されており、コメントを外しても新Providerやフォールバックは有効になりません。将来の既定順序はCerebras → Groq → OpenRouter Freeで、複数ProviderモードでxAIを利用するには `AI_PROVIDER_ORDER` に `xai` を明示します。モデル・無料枠・有料化条件は導入時に公式情報で確認してください。

設定後に起動します。初回やソース・ビルド時設定を変更した場合は、先にビルドします。

```sh
docker compose -f compose.prod.yml build
docker compose -f compose.prod.yml up -d
```

ルートの `.env` は自動で読み込まれます。既存の `.env.prod` を使い続ける場合は両コマンドに `--env-file .env.prod` を指定し、バックエンド用リンクもそのファイルを指すようにしてください。これらの実値ファイルは `.gitignore` で除外済みです。

本番Composeは、デフォルトではフロントエンドとバックエンドを `127.0.0.1` にのみ公開し、PostgreSQLはホストへ公開しません。外部公開にはリバースプロキシやトンネルを前段に配置してください。
