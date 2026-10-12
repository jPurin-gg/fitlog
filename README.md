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

## CI（GitHub Actions）

`.github/workflows/ci.yml` は、全baseブランチのPR作成・追加コミット・再オープン・編集（base変更を含む）、mainへのpush、手動実行で動きます。PRではcheckout既定のマージ結果を検証します。作業ブランチへのpushだけでは実行しません。同じPRの古い実行だけをキャンセルし、main pushと手動実行は実行IDごとに分離します。

Ubuntu 24.04で次の4ジョブを並列実行し、`ci-required` に集約します。Node.jsはDockerと同じ22系（22.14以上）、Goは `backend/go.mod` のバージョンを使います。npmは `frontend/package-lock.json`、Goは `backend/go.sum` に対応するキャッシュを利用します。

| ジョブ | 検証コマンド | 上限 |
| --- | --- | --- |
| `frontend` | `npm ci` → `npm run lint` → `npx tsc --noEmit` → `npm test` → `npm run build` | 15分 |
| `backend` | `go test -race ./...` → `go vet ./...` | 15分 |
| `db-integration` | `go test -race -tags=integration -count=1 -json ./internal/app ./internal/workout/postgres` → 結果検査 | 20分 |
| `e2e` | `npm ci` → `npx playwright install --with-deps chromium` → `npm run test:e2e -- --workers=1 --reporter=list,html` | 25分 |

各コマンドは対応する `frontend` / `backend` ディレクトリで実行します。DB統合テストはPostgreSQL 16の一時service containerに、明示した `FITLOG_TEST_DATABASE_DSN` で接続します。接続情報はCI専用のダミー値です。JSON結果からskip・失敗・未完了のスイートを拒否し、両スイートの代表的な統合テストが通ったことも確認します。代表テストを改名する際は `.github/scripts/check_integration_tests.py` の `REQUIRED_TESTS` も更新してください。実行するテスト件数は固定しません。

E2Eは既存のNext.js devサーバーとAPIモックを使います。`CI=true`、worker 1、retry 2回、全体20分、サーバー起動120秒で実行し、専用Chromiumを導入するため `PLAYWRIGHT_CHANNEL` は指定しません。再試行1回目にtraceを記録します。通常のCIは `contents: read` 権限と合成データだけで実行でき、外部AIや本番シークレットは不要です。

### 失敗の調査

PRのChecks、またはActions → CIから失敗したジョブとステップのログを確認します。DBが接続できない場合はserviceのhealthcheckとDSN、型検査・ビルドの場合は新規checkoutでの再現を確認してください。集約が失敗した場合は依存4ジョブの結果を確認します。failure・cancelled・skippedはすべて不合格です。

E2E失敗時はActionsの実行画面のArtifactsから `playwright-failure-<run_id>-<run_attempt>` を取得できます（保持7日）。ZIPを展開し、frontendで次のように開きます。

```sh
npx playwright show-report /path/to/artifact/playwright-report
npx playwright show-trace /path/to/artifact/test-results/<test>-retry1/trace.zip
```

HTMLレポートから失敗内容と添付traceを確認できます。ブラウザ導入などテスト前の失敗ではレポートが生成されない場合があり、その場合はジョブログを使います。CI中のHTML自動起動は無効です。

CIの結果判定自体は、リポジトリルートで `python3 -B -m unittest discover -s .github/scripts -p 'test_*.py'` により検証できます。各ジョブのfailure・cancelled・skipped、DBテストのskip・結果欠落を拒否することを確認します。

### 初期導入と必須チェック

CIファイルの追加だけではマージを制限しません。次の順で導入し、設定と実測結果を [issue #37](https://github.com/jPurin-gg/fitlog/issues/37) に記録します。ワークフロー実装の検証とGitHub側の設定完了は分けて扱います。

1. CI追加PRで全ジョブの成功を確認します。PRの更新・再オープン、main以外のbase、baseをmainへ変更した後のマージSHA、同じPRの古い実行のキャンセルも確認します。
2. mainへのマージ後、main pushでの実行とActions → CI → Run workflowでの手動実行を確認します。連続するmain pushが互いをキャンセルしないことも確認します。手動実行はデフォルトブランチへの導入後に利用でき、ルールセットのPR必須チェックの代用にはしません。
3. 管理者がSettings → Branchesでmainを対象にブランチ保護を作成します。Require a pull request before merging、Require status checks to pass before mergingを有効にし、チェック `ci-required`（GitHub Actions）を選択します。Do not allow bypassing the above settingsも有効にし、管理者も通常のPRとCIを通します。直近7日以内に成功したチェックを候補に表示させてから設定します。
4. main向けの失敗する検証用PRで `ci-required` が失敗し、マージがブロックされることを確認します。管理者バイパスを利用せず、失敗コミットはmainへマージしません。E2Eも意図的に失敗させ、ダウンロードしたHTMLと再試行traceを確認します。
5. 設定方式・対象main・設定者・設定日時・`ci-required`・管理者/バイパスの扱い、および検証PRとActions実行URLをissueに残します。権限やプランで必須化できない場合は制約と代替のレビュー運用を記録し、「マージを止める」条件は未達として残します。

`ci-required` は他のワークフローと重複させない固定名です。改名時は新旧両チェックを一時的に出し、新名の成功後にmainの保護設定へ追加し、旧名を必須チェックから外してから旧ジョブを削除します。必須チェックを存在しない名前にしないでください。Merge queueを導入する場合は先に `merge_group` イベントへ対応し、必須チェックを再検証します。

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
