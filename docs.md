# Fitlog 技術仕様

## アーキテクチャ

Fitlogのバックエンドは、機能単位に分割したモジュラーモノリスです。MVCのレイヤーをアプリ全体へ横断的に置くのではなく、`auth` や `workout` などの機能ごとに以下を閉じ込めます。

```text
HTTP request
  -> feature/httpapi Handler
  -> feature Service（入力検証・ユースケース）
  -> feature Repository / AI interface
  -> feature/postgres または ai/openaicompat
```

共通基盤は次の責務だけを持ちます。

- `apperr`: アプリケーションエラー
- `httpx`: JSON、Problem Details、request ID、CORS、ログ、panic recovery
- `config`: 環境変数の検証
- `database`: DB接続とバージョン付きマイグレーション
- `ai`: AIタスク非依存のポートとエラー変換
- `prompt`: ファイルベースのプロンプト描画
- `clock`: 日付依存処理の境界

ハンドラーはHTTP変換だけ、Serviceはユースケースだけ、PostgreSQL実装は永続化だけを担当します。機能間の共有は、具体Repositoryではなく小さな読み取りインターフェースを介します。

## 認証

`POST /api/auth/login` はニックネームを一意キーとして検索します。

- 未登録ならユーザーを自動作成
- 登録済みならPBKDF2-SHA256でパスワード照合
- 旧データで `password_hash` が空の場合は、初回ログイン時にパスワードを設定
- 成功時にHMAC-SHA256署名済みCookieを発行
- Cookieは `HttpOnly`、`SameSite=Lax`、既定30日
- CookieにはユーザーIDと期限だけを含め、サーバー側セッションテーブルは持たない

すべての保護APIはCookieからユーザーIDを取得します。URLやJSONに `user_id` を指定して別ユーザーへアクセスする経路はありません。

サーバー側セッションを持たないため、ブラウザからのログアウトはCookieを削除する操作です。別の場所にコピーされた個別トークンの失効はできず、`SESSION_SECRET` のローテーションで全セッションが一括失効します。本番ではHTTPSと `SESSION_COOKIE_SECURE=true` を必須とします。

## HTTP契約

成功時はエンベロープを付けずJSONを直接返します。削除など本文が不要な操作は `204 No Content` です。

エラーはRFC 9457形式の `application/problem+json` です。

```json
{
  "type": "/problems/validation-error",
  "title": "Invalid request",
  "status": 400,
  "detail": "セット記録の入力を確認してください。",
  "code": "VALIDATION_ERROR",
  "request_id": "...",
  "errors": {
    "reps": "must be positive"
  }
}
```

JSON入力は1 MiBまでで、未知フィールドと複数JSON値を拒否します。

## API一覧

### 認証

| Method | Path | 説明 |
|---|---|---|
| POST | `/api/auth/login` | ログインまたは自動登録 |
| GET | `/api/auth/me` | 現在のユーザー |
| DELETE | `/api/auth/session` | ログアウト |

ログイン入力:

```json
{ "nickname": "Mitsuki", "password": "password" }
```

### ダッシュボード・設定

| Method | Path | 説明 |
|---|---|---|
| GET | `/api/dashboard` | 直近統計と履歴 |
| GET | `/api/calendar?year=2026&month=8` | 月間の実施日・予定日 |
| GET | `/api/preferences` | AIコーチ設定 |
| PUT | `/api/preferences` | AIコーチ設定を保存 |

### 種目

| Method | Path | 説明 |
|---|---|---|
| GET | `/api/exercises` | `name`、`muscle`、`equipment`、`level` で検索 |
| POST | `/api/exercises` | カスタム種目を作成 |
| GET | `/api/exercises/recent` | 最近使った種目 |
| GET | `/api/exercises/favorites` | お気に入り一覧 |
| PUT | `/api/exercises/{exerciseID}/favorite` | お気に入りに追加 |
| DELETE | `/api/exercises/{exerciseID}/favorite` | お気に入りから削除 |
| GET | `/api/exercises/{exerciseID}/settings` | 目標セット数 |
| PUT | `/api/exercises/{exerciseID}/settings` | 目標セット数を保存 |
| POST | `/api/exercises/{exerciseID}/alternatives` | AIによる代替種目候補 |

### 月間・日次プラン

| Method | Path | 説明 |
|---|---|---|
| GET | `/api/monthly-plans` | 保存済み月間プラン一覧 |
| GET | `/api/monthly-plans/{YYYY-MM}` | 指定月のプラン |
| PUT | `/api/monthly-plans/{YYYY-MM}` | 指定月のプランを保存 |
| POST | `/api/monthly-plans/{YYYY-MM}/generate` | AIで生成して保存 |
| GET | `/api/workout-plans/{YYYY-MM-DD}` | 保存済み予定、なければ月間プランからdraftを構築 |
| PUT | `/api/workout-plans/{YYYY-MM-DD}` | 日次予定を保存 |
| POST | `/api/workout-plans/{YYYY-MM-DD}/start` | 日次予定を確定してワークアウト開始 |

開始APIはアプリ設定のタイムゾーンにおける当日だけ受け付けます。過去・未来の記録編集は `/api/workouts/by-date/{YYYY-MM-DD}` を使います。

### ワークアウト

| Method | Path | 説明 |
|---|---|---|
| GET | `/api/workouts/by-date/{YYYY-MM-DD}` | 指定日の記録 |
| PUT | `/api/workouts/by-date/{YYYY-MM-DD}` | 指定日の記録を置換保存 |
| GET | `/api/workouts/{workoutID}` | 記録詳細と集計 |
| POST | `/api/workouts/{workoutID}/sets` | 1セット保存 |
| POST | `/api/workouts/{workoutID}/sets/{setID}/recommendation` | 保存済みセットを基にAI提案 |
| POST | `/api/workouts/{workoutID}/finish` | 終了・集計・AI総評 |

セット保存には8〜128文字のURL-safeな `Idempotency-Key` ヘッダーが必須です。同じキーと同じ本文の再送は既存セットを返し、異なる本文で同じキーを使うと `409 Conflict` です。これによりAI呼び出し失敗後の再試行でもセットが二重登録されません。

## AI境界

機能Serviceは `ai.Client` だけを参照し、HTTP APIのURLや認証方式を知りません。現在の具体実装はOpenAI互換Chat Completionsアダプター1つです。

- 設定名は `OPENAI_API_KEY`、`OPENAI_API_URL`、`OPENAI_MODEL`
- JSONが必要なタスクでは `response_format: json_object`
- 429、408、5xxのみ最大3回まで再試行
- `Retry-After` と指数バックオフを尊重
- ローカルRPM制限あり
- プロンプト、APIキー、ユーザーデータをログへ出さない
- AI失敗時はProblem Detailsへ変換し、セット保存結果は失わない

プロバイダーを追加する場合も、機能Serviceではなく `internal/ai` 配下へ新しいアダプターを追加します。現時点では自動ルーティングや複数キー管理は行いません。

## データベース変更

マイグレーションは `backend/internal/database/migrations` に昇順のSQLファイルとして追加します。起動時に次の手順で適用します。

1. `schema_migrations` を作成
2. PostgreSQLアドバイザリロックを取得
3. 未適用ファイルを1ファイル1トランザクションで実行
4. 適用済みファイル名を記録

現在の移行では、既存テーブルをベースラインとして認識し、ユーザー名一意インデックスとセットの冪等キーを追加します。重複ユーザー名が存在するDBでは、一意化を勝手に行わずマイグレーションを停止します。

## テスト方針

- `auth`: パスワード、署名Cookieトークン、登録・旧ユーザー移行
- `ai/openaicompat`: リクエスト形式、再試行、非再試行、レート制限、壊れたレスポンス
- `planning`: 休息日ポリシー、AIが辞書外IDを返した場合の拒否
- `profile`: 設定正規化
- `prompt`: 全テンプレートの描画
- `workout`: 冪等キー入力契約
- Docker上で `go test`、race detector、`go vet`、ESLint、TypeScript検査、Next.js本番ビルド
- npm本番依存は `npm audit --omit=dev` で監査
