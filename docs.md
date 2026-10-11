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
| POST | `/api/workout-plans/{YYYY-MM-DD}/start` | AI利用を選択して開始、または種目指定で手動開始 |

開始APIはアプリ設定のタイムゾーンにおける当日だけ受け付けます。過去・未来の記録編集は `/api/workouts/by-date/{YYYY-MM-DD}` を使います。開始リクエストは任意のJSON本文を受け付けます。

```json
{ "use_ai": false, "exercise_id": "bench_press" }
```

| フィールド | 動作 |
|---|---|
| `use_ai: false` | AI調整を呼び出さず開始。日次予定がなければ月間プランから基礎メニューを構築 |
| `use_ai: true` | 日次予定がない場合にAI調整を試み、失敗時は基礎メニューで開始 |
| `use_ai` 省略・`null`、または本文なし | 従来クライアントとの互換性のためONと同じ動作 |
| `exercise_id` | 手動で選択した種目。月間プランを参照せず、`use_ai` にかかわらずAI調整なしで開始 |

手動開始は有効な種目IDが必要です。日次予定がなければその種目の予定を作り、既存の日次予定があれば予定とワークアウトをそのまま再利用します。クライアントは開始前に日次予定をPUTする必要がありません。種目指定なしで月間・日次プランの両方がない場合は404を返し、画面は手動選択へ進みます。DBの障害は未作成とは区別してエラーにします。不正JSON・未知のフィールド・型が異なる値は拒否します。

開始用の予定保存は `CreateDailyIfMissing` による作成専用です。ワークアウトの紐付けは日次予定の行ロックで直列化し、同時開始では同じ `workout_id` を返します。AIの応答待ち中に別の開始や手動編集が予定を確定しても、後着の開始処理では上書きしません。日次予定の明示的な編集は引き続きPUTで行います。

`PlanSession.ai_status` は `applied`（AI調整を反映）、`fallback`（AIに失敗して基礎プランを反映）、`not_requested`（明示OFF・手動開始・保存済み予定の再利用）のいずれかです。競合した開始が先に予定を作成していた場合も、その予定を再利用して `not_requested` を返します。

### ワークアウト

| Method | Path | 説明 |
|---|---|---|
| GET | `/api/workouts/by-date/{YYYY-MM-DD}` | 指定日の記録 |
| PUT | `/api/workouts/by-date/{YYYY-MM-DD}` | 指定日の記録を置換保存 |
| GET | `/api/workouts/{workoutID}` | 記録詳細と集計 |
| POST | `/api/workouts/{workoutID}/sets` | 1セット保存 |
| POST | `/api/workouts/{workoutID}/sets/{setID}/recommendation` | 保存済みセットを基にAI提案 |
| POST | `/api/workouts/{workoutID}/finish` | 終了とDB集計（AIは呼び出さない） |
| POST | `/api/workouts/{workoutID}/summary-comment` | 完了済み記録のAI総評を生成。保存済みなら再利用 |

セット保存には8〜128文字のURL-safeな `Idempotency-Key` ヘッダーが必須です。同じキーと同じ本文の再送は、ワークアウト終了後でも既存セットを返します。異なる本文で同じキーを使うと `409 Conflict` です。
クライアントは保存成否を判定できない通信失敗時に入力と画面遷移を固定し、同じキーと本文で再送します。

セットの新規保存と終了は同じworkout行をロックして直列化します。保存が先なら終了集計に必ず含まれ、終了が先なら新規保存を拒否します。

AI総評は `{ "comment": "...", "replayed": false }` を返します。同じワークアウトの保存済み総評がある場合は `replayed: true` です。未完了のワークアウトは `409 Conflict` です。AI障害でこのAPIが失敗しても、先に完了した記録とDB集計は変更しません。

## AI境界

フロントエンドは `AISettingsProvider` でユーザーごとの設定を共有します。`localStorage` の `fitlog:ai-enabled:{userID}` に `true` / `false` を保存し、初期値・読み込み前・不正値・読み込み例外時はOFFです。画面をまたいで同じトグルを使い、認証ユーザーが変わると状態を切り替えます。

OFFでは開始リクエストに `use_ai: false` を送り、セット保存後の提案・終了後の総評・月間生成・代替種目相談の呼び出しを止めます。待機中のAIはAbortControllerで中断し、リクエスト識別子と最新設定で後着結果を破棄します。通常のセット保存は中断せず、入力・セット番号・保存状態・冪等キーを維持します。ONに戻しても保存済みの記録を再送しません。終了後は明示ボタンで総評を取得できます。すでに送信したAI処理の撤回や、完了済み保存の取り消しは保証しません。

機能Serviceは `ai.Client` だけを参照し、HTTP APIのURLや認証方式を知りません。現在の具体実装はOpenAI互換Chat Completionsアダプター1つです。

- xAI向け設定名は `XAI_API_KEY`、`XAI_API_URL`、`XAI_MODEL`
- JSONが必要なタスクでは `response_format: json_object`
- 429、408、5xxのみ最大3回まで再試行
- `Retry-After` と指数バックオフを尊重
- ローカルRPM制限あり
- 記録に付随するAI処理は `AI_OPTIONAL_TIMEOUT_SECONDS`（既定15秒）で上限時間を持つ
- プロンプト、APIキー、ユーザーデータをログへ出さない
- AI失敗時はProblem Detailsへ変換し、セット保存結果は失わない

HTTPログは `request_id`、method、生のIDを含まないroute pattern、status、response bytes、所要時間を記録します。AIログは同じ `request_id` で相関でき、provider段階の最終outcome・試行回数・総時間と、feature段階の `applied`、`fallback`、`invalid_output`、`unavailable`、`replayed` などを分けて記録します。

プロバイダーを追加する場合も、機能Serviceではなく `internal/ai` 配下へ新しいアダプターを追加します。現時点では自動ルーティングや複数キー管理は行いません。

## 月間プランの案内

ホームでは当月の月間プランが404で、非表示設定の読み込みが完了し、当月が非表示でない場合に案内を自動表示します。AI利用設定とは独立した判定で、案内の表示だけでは生成しません。プラン取得が404以外で失敗した場合や、当月のプランがある場合は自動表示しません。

「今月は次回以降表示しない」は `localStorage` の `fitlog:monthly-prompt-dismissed:{userID}` に対象年月を保存します。月の形式・基準はプラン取得と同じ `formatPlanMonth`（ブラウザのローカル年月の `YYYY-MM`）です。当月との完全一致だけを非表示とし、翌月・年越し・別ユーザーには持ち越しません。ストレージの読み書きができない場合も今回の案内は閉じられます。

通常の閉じる操作では保存値を変更しません。同じホーム表示中は開き直さず、次の訪問で再判定します。ホームの「月間プランを作成／見直す」は非表示中も利用でき、開閉しても非表示設定を解除しません。ホームを開いたままの月またぎは画面復帰・可視状態の変更・1分ごとの時刻確認で判定します。記録画面では自動案内を行いません。

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
- `planning`: 休息日ポリシー、AIが辞書外IDを返した場合の拒否、開始オプション、AIなしの手動開始と既存予定保持
- `profile`: 設定正規化
- `prompt`: 全テンプレートの描画
- `workout`: 冪等キー入力契約
- 専用の一時PostgreSQLで、同一セットの同時再送、終了と保存の競合、終了後リプレイを検証
- `app` のHTTP統合テストで認証→月間・日次プランなしの手動開始→複数セット・種目変更→終了→履歴取得を検証。ローカルのAI用サーバーで呼び出し0件を確認し、8件の並行開始と遅いAIによる上書き防止も検証
- React Testing LibraryとMSWで保存状態とAI状態の分離、保存・提案・総評の待機中のOFF切り替え、入力保持、遅い応答の破棄を検証
- 時刻・ストレージを制御し、月間案内の当月非表示、通常の閉じる、再訪、翌月、年越し、別ユーザー、手動表示を検証
- PlaywrightでAI OFFの手動記録・終了とAI呼び出し0件、AI ONの障害時継続、月間案内の非表示・再表示と設定共有をブラウザ検証
- Docker上で `go test`、race detector、`go vet`、ESLint、TypeScript検査、Next.js本番ビルド
- npm本番依存は `npm audit --omit=dev` で監査
