# fitlog 仕様書

## 概要

個人向けの筋トレ記録 × AI コーチングアプリ。  
Docker Compose で Next.js / Go / PostgreSQL を一括起動するローカルファースト設計。  
Gemini 2.5 Flash（OpenAI互換エンドポイント）を AI バックエンドとして利用。

---

## 技術スタック

| レイヤー       | 技術                              |
|-------------|-----------------------------------|
| フロントエンド | Next.js 14 (App Router / TypeScript / Tailwind CSS) |
| バックエンド   | Go 1.22（標準 `net/http`、`air` でホットリロード）|
| データベース   | PostgreSQL 16                     |
| AI           | Google Gemini 2.5 Flash（OpenAI互換 API）|
| コンテナ      | Docker Compose v2                 |

---

## ディレクトリ構成

```
fitlog/
├── compose.yaml              # Docker Compose 定義
├── docs.md                   # 本仕様書
├── frontend/                 # Next.js アプリ
│   ├── app/
│   │   ├── page.tsx          # ホーム（ダッシュボード）
│   │   ├── workout/page.tsx  # 記録する
│   │   ├── exercises/page.tsx# 辞書（種目ライブラリ）
│   │   └── calendar/page.tsx # カレンダー
│   └── components/
│       ├── BottomNav.tsx           # 下部ナビゲーション
│       ├── ExerciseSelectorModal.tsx # 種目選択モーダル
│       └── AlternativeCoachModal.tsx # AI代替種目提案モーダル
├── backend/                  # Go サーバー
│   ├── main.go               # エントリポイント・ルーティング
│   ├── db.go                 # DB接続
│   ├── seed.go               # 初回 DB シーディング
│   ├── handlers.go           # /api/recommend
│   ├── api_exercises.go      # /api/exercises, /api/exercises/target_sets
│   ├── api_custom_exercise.go# /api/exercises/custom
│   ├── api_extra.go          # /api/dashboard, /api/calendar, /api/alternative
│   ├── api_ai.go             # AI APIクライアント（callAI関数）
│   └── tmpkin_jp.json        # 種目マスターデータ（873件・日本語）
└── docker/
    ├── db/init.sql            # DB スキーマ定義
    ├── backend.Dockerfile
    ├── frontend.Dockerfile
    └── env/
        ├── backend.env        # バックエンド環境変数（APIキー含む）
        ├── frontend.env
        └── db.env
```

---

## データベーススキーマ

### `users`
| カラム       | 型                        | 説明             |
|------------|--------------------------|-----------------|
| id         | SERIAL PRIMARY KEY        |                 |
| username   | TEXT NOT NULL             |                 |
| created_at | TIMESTAMPTZ DEFAULT NOW() |                 |

> 現在は `id=1` の固定ユーザー（Mitsuki）のみ。マルチユーザー非対応。

### `exercises`（種目マスター）
| カラム             | 型     | 説明                           |
|-----------------|--------|-------------------------------|
| id              | TEXT PK| 例: `Barbell_Bench_Press_-_Medium_Grip` |
| name            | TEXT   | 日本語種目名                    |
| force           | TEXT   | push / pull / static 等        |
| level           | TEXT   | beginner / intermediate / expert |
| mechanic        | TEXT   | compound / isolation           |
| equipment       | TEXT   | ダンベル / バーベル 等           |
| category        | TEXT   | 筋力トレーニング 等              |
| instructions    | JSONB  | 手順テキスト配列                 |
| primary_muscles | JSONB  | 主要筋肉配列（日本語）           |
| secondary_muscles | JSONB | 補助筋肉配列                   |
| images          | JSONB  | 画像パス配列（未使用）           |

> カスタム種目は `id = "custom_<UnixNano>"` で同テーブルに保存。

### `user_exercise_stats`（ユーザー種目統計）
| カラム        | 型            | 説明                    |
|-------------|--------------|------------------------|
| user_id     | INTEGER (FK)  |                        |
| exercise_id | TEXT (FK)     |                        |
| weight      | FLOAT         | 直近の重量（kg）          |
| max_weight  | FLOAT         | 過去最大重量（1RM参考値）  |
| target_sets | INTEGER DEFAULT 3 | 目標セット数          |
| updated_at  | TIMESTAMPTZ   |                        |

### `workouts`（ワークアウトセッション）
| カラム      | 型          | 説明                         |
|-----------|------------|------------------------------|
| id        | SERIAL PK  |                              |
| user_id   | INTEGER FK |                              |
| started_at| TIMESTAMPTZ| セッション開始時刻              |
| ended_at  | TIMESTAMPTZ| セッション終了時刻（NULL=継続中）|
| notes     | TEXT       | メモ（未入力時 NULL）          |

### `workout_sets`（セット記録）
| カラム      | 型          | 説明                     |
|-----------|------------|--------------------------|
| id        | SERIAL PK  |                          |
| workout_id| INTEGER FK |                          |
| exercise_id| TEXT FK   |                          |
| weight    | FLOAT NOT NULL |                      |
| reps      | INTEGER NOT NULL |                    |
| set_order | INTEGER NOT NULL | セット番号（1始まり）   |
| feeling   | TEXT       | 感想（例: 余裕あり、限界）  |
| is_pr     | BOOLEAN DEFAULT FALSE | 自己ベスト更新フラグ |
| created_at| TIMESTAMPTZ|                          |

---

## API エンドポイント一覧

### `POST /api/recommend`
セット完了時に呼び出す。DBへの記録保存 + AI によるアドバイス生成を行う。

**リクエスト**
```json
{
  "user_id": 1,
  "exercise_id": "Barbell_Bench_Press_-_Medium_Grip",
  "set_order": 2,
  "weight": 80.0,
  "reps": 10,
  "feeling": "まだ余裕がある"
}
```

**処理フロー**
1. `user_exercise_stats` から過去最大重量を取得
2. 当日の `workouts` レコードを検索（なければ新規作成）
3. `workout_sets` にセットを INSERT
4. `user_exercise_stats` を UPSERT（max_weight を更新）
5. Gemini にプロンプトを投げ、次のセットを提案

**レスポンス**
```json
{
  "next_action": "CONTINUE",
  "recommendation": "調子が良さそうです！重量を少し上げてみましょう。",
  "target_weight": 82.5,
  "target_reps": 8,
  "reason": "余裕があり、過去最大重量に迫っているため。",
  "max_weight": 85.0
}
```

> `next_action` は `"CONTINUE"` または `"STOP"`。AI 呼び出しが失敗した場合はルールベースのフォールバックロジックが動作する。

---

### `GET /api/exercises`
種目マスターデータを取得。クライアント側でも絞り込みは行われるが、サーバー側でも以下のフィルタが使える。

| クエリパラメータ | 説明                      |
|-------------|--------------------------|
| muscle      | 筋肉名（LIKE 検索）         |
| equipment   | 器具名（完全一致）           |

**レスポンス**: `Exercise[]`（最大 100 件）

---

### `POST /api/exercises/custom`
ユーザーが独自種目を追加する。

**リクエスト**
```json
{
  "name": "マイオリジナル種目",       // 必須
  "category": "筋力トレーニング",     // 任意
  "equipment": "ダンベル",           // 任意
  "primary_muscle": "大胸筋"         // 任意
}
```

> `name` が空の場合は 400 エラー。それ以外は NULL 保存。SQL インジェクション対策としてすべてプレースホルダー使用。

---

### `GET /api/exercises/target_sets`
種目ごとの目標セット数を取得。

| クエリパラメータ | 説明     |
|-------------|---------|
| user_id     | ユーザーID |
| exercise_id | 種目ID   |

**レスポンス**: `{ "target_sets": 4 }`（レコードなし時はデフォルト 3）

### `POST /api/exercises/target_sets`
目標セット数を保存（UPSERT）。

```json
{ "user_id": 1, "exercise_id": "...", "target_sets": 4 }
```

---

### `POST /api/alternative`
現在の種目の代替種目を AI が提案する。

**リクエスト**
```json
{
  "exercise_id": "Barbell_Bench_Press_-_Medium_Grip",
  "exercise": "ベンチプレス",
  "reason": "マシンが空いていない"
}
```

**処理フロー**
1. `exercise_id` の `primary_muscles` を DB から取得
2. 同じ筋肉を鍛えられる他の種目を最大 30 件検索
3. 候補リストと理由を Gemini に渡し、2〜3 件に絞らせる
4. AI 失敗時はキーワードベースのフォールバック

**レスポンス**
```json
{
  "message": "以下の種目で同じ部位を追い込みましょう！",
  "alternatives": [
    { "id": "Dumbbell_Bench_Press", "name": "ダンベルベンチプレス", "description": "..." }
  ]
}
```

---

### `GET /api/dashboard`
ホーム画面向けデータ。最近のワークアウト3件 + 過去7日間のセット数を返す。

**レスポンス**
```json
{
  "stats": [...],
  "chart_data": [0, 0, 3, 5, 0, 8, 2],
  "recent_workouts": [
    { "id": 1, "title": "Workout Session", "type": "Strength", "duration": "45 min", "calories": "225 kcal", "time": "07:30 PM" }
  ]
}
```

> `stats`（カロリー、心拍数等）はモックデータ。

### `GET /api/calendar`
カレンダー表示用。指定月のワークアウト実施日を返す。

| クエリパラメータ | 説明          |
|-------------|-------------|
| year        | 年（省略時: 今年）|
| month       | 月（省略時: 今月）|

---

## AI 連携仕様

### 接続先
`docker/env/backend.env` の環境変数で制御。

```env
OPENAI_API_KEY=<Gemini APIキー>
OPENAI_API_URL=https://generativelanguage.googleapis.com/v1beta/openai/chat/completions
OPENAI_MODEL=gemini-2.5-flash
```

> Gemini の OpenAI 互換エンドポイントを使用しているため、コードは OpenAI SDK 互換の形式でそのまま動作する。

### フォールバック
AI 呼び出しが失敗した場合（APIキー未設定、レート制限等）、ルールベースロジックで代替レスポンスを生成する。アプリはクラッシュしない。

---

## フロントエンド画面仕様

### ホーム（`/`）
- 今日のワークアウト概要
- 過去7日間のセット数グラフ
- 最近のワークアウト履歴3件

### 記録する（`/workout`）
1. **種目選択画面**：デフォルト「ベンチプレス」で Start Workout
2. **記録画面**：
   - 種目名（タップで変更 or AI代替提案）
   - `SET X / 目標Y` カウンター + 進捗ドット
   - 目標セット数の編集（± ボタン、DB に自動保存）
   - 重量・回数・感想入力
   - **「Set Completed - Analyze」** ボタン → AI アドバイス表示
   - CONTINUE の場合：次のセット目標（重量 / 回数）を表示
   - STOP の場合：赤いストップ表示

### 辞書（`/exercises`）
- 873件の種目をブラウジング
- キーワード検索
- 部位フィルタ（単一選択）、器具フィルタ（複数選択）
- アクティブフィルターバーがスクロールに追従（sticky）
- 種目タップで詳細モーダル（筋肉 / 手順 / 器具）

### カレンダー（`/calendar`）
- 月次カレンダー
- ワークアウト実施日にドット表示

### 共通コンポーネント

**`ExerciseSelectorModal`**
- `/api/exercises` から全件取得してクライアント側フィルタ
- ざっくり部位（胸 / 背中 / 脚…）→ 詳細筋肉の2段階フィルタ連動
- 器具フィルタ（複数選択）
- カスタム種目追加フォーム（名前必須）

**`AlternativeCoachModal`**
- 理由入力 → `/api/alternative` 呼び出し
- 返ってきた候補をタップすると即座に種目切り替え
- 手動入力でも種目を変更可能

---

## 起動方法

```bash
# 初回のみ: docker/env/backend.env の OPENAI_API_KEY に Gemini APIキーを設定
docker compose up -d

# バックエンドのみ再起動（環境変数変更後）
docker compose up -d backend

# ログ確認
docker logs --tail 50 myfitlog-backend
```

- フロントエンド: http://localhost:3000
- バックエンド: http://localhost:8080

---

## 環境変数

### `docker/env/backend.env`

| 変数名             | デフォルト値                                                                 | 説明               |
|-----------------|----------------------------------------------------------------------------|------------------|
| PORT            | 8080                                                                        | リッスンポート      |
| DB_HOST         | db                                                                          | PostgreSQLホスト  |
| DB_PORT         | 5432                                                                        |                   |
| DB_USER         | myfitlog                                                                    |                   |
| DB_PASSWORD     | myfitlog                                                                    |                   |
| DB_NAME         | myfitlog                                                                    |                   |
| OPENAI_API_KEY  | *(要設定)*                                                                   | Gemini APIキー    |
| OPENAI_API_URL  | `https://generativelanguage.googleapis.com/v1beta/openai/chat/completions` |                   |
| OPENAI_MODEL    | gemini-2.5-flash                                                            |                   |
| FRONTEND_URL    | http://localhost:3000                                                       | CORS 許可オリジン  |
| SEED_FILE_PATH  | tmpkin_jp.json                                                              | 種目JSONのパス    |

---

## セキュリティ

- SQL インジェクション対策: 全 DB クエリでプレースホルダー（`$1`, `$2`...）を使用
- CORS: `FRONTEND_URL` で指定したオリジンのみ許可
- API キー: コンテナの環境変数で注入（ソースコードに直書きなし）
- 入力バリデーション: 必須フィールドのみサーバー側で検証、任意フィールドは NULL 保存

---

## 既知の制限・今後の改善候補

| 項目                  | 現状                                       | 改善案                                  |
|--------------------|------------------------------------------|----------------------------------------|
| マルチユーザー         | `user_id = 1` ハードコード                 | 認証実装 + ユーザー管理                  |
| `stats` (カロリー等)  | ホーム画面はモックデータ                    | 実記録から計算                           |
| 種目検索              | クライアント側フィルタ（全件取得後）         | サーバー側全文検索（pgベクトル等）        |
| workout 終了         | `ended_at` が常に NULL（終了ボタンなし）   | 「ワークアウト終了」ボタンの実装          |
| AI プロンプト         | バックエンドにハードコード                  | 管理画面または設定ファイルで変更可能にする |
| PWA / モバイル対応    | ブラウザのみ                               | manifest.json / Service Worker 追加    |

---

## 開発用スクリプト（移行・翻訳ツール、本番不要）

以下のファイルは開発・移行時のみ使用したスクリプトです。**アプリの動作には不要**で、将来的にはリポジトリから除外しても問題ありません。

| ファイル                    | 用途                                                           |
|--------------------------|--------------------------------------------------------------|
| `backend/translate_all.py`| `tmpkin_en.json` → `tmpkin_jp.json` へ Google Translate で一括翻訳 |
| `backend/make_sql.py`     | 翻訳済みJSON から UPDATE SQL を生成（`update_translations.sql`）   |
| `backend/check_progress.py`| `tmpkin_jp.json` の翻訳進捗チェック（日本語文字の含有率を計算）  |
| `backend/update_translations.sql` | 翻訳適用用 SQL（約 873件の UPDATE 文、829KB）             |
| `backend/translation_log.txt` | 翻訳スクリプト実行時のログ                                    |
| `backend/kin.json`        | 翻訳前の小規模サンプルJSON（動作確認用）                         |
| `backend/tmpkin_en.json`  | 翻訳前の英語種目データ原本（1MB）                               |
| `backend/myfitlog-backend` | ローカルビルド成果物（バイナリ）。Docker 内ビルドが優先されるため不要 |
| `backend/cmd/`、`backend/handlers/`、`backend/models/` | 空ディレクトリ（初期プロジェクト生成時の残骸） |
| `backend/tmp/`            | `air` のビルドキャッシュ（`.gitignore` 済み）                   |
