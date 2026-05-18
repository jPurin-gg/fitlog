-- ユーザー
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    password_hash TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 種目 (マスター辞書データ)
CREATE TABLE exercises (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    force TEXT,
    level TEXT,
    mechanic TEXT,
    equipment TEXT,
    category TEXT,
    instructions JSONB,
    primary_muscles JSONB,
    secondary_muscles JSONB,
    images JSONB
);

-- ユーザーごとの種目データ（重量・最大重量など）
CREATE TABLE user_exercise_stats (
    user_id INTEGER REFERENCES users(id),
    exercise_id TEXT REFERENCES exercises(id),
    weight FLOAT,
    max_weight FLOAT,
    target_sets INTEGER DEFAULT 3,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, exercise_id)
);

-- ワークアウトセッション
CREATE TABLE workouts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP WITH TIME ZONE,
    notes TEXT,
    summary_comment TEXT
);

-- セットごとの記録
CREATE TABLE workout_sets (
    id SERIAL PRIMARY KEY,
    workout_id INTEGER REFERENCES workouts(id),
    exercise_id TEXT REFERENCES exercises(id),
    weight FLOAT NOT NULL,
    reps INTEGER NOT NULL,
    set_order INTEGER NOT NULL,
    feeling TEXT, -- セット後の感想
    is_pr BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 月間トレーニングプラン
CREATE TABLE monthly_plans (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    plan_month TEXT NOT NULL, -- 例: 2026-05
    plan_name TEXT NOT NULL,
    frequency TEXT NOT NULL,
    description TEXT,
    rationale TEXT,
    rest_days JSONB NOT NULL DEFAULT '[]'::jsonb,
    recommended_days JSONB NOT NULL DEFAULT '[]'::jsonb,
    weekly_routine JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, plan_month)
);

-- 日次ワークアウト計画
CREATE TABLE workout_plans (
    id SERIAL PRIMARY KEY,
    workout_id INTEGER REFERENCES workouts(id),
    user_id INTEGER REFERENCES users(id),
    plan_date DATE NOT NULL,
    title TEXT NOT NULL,
    estimated_duration_min INTEGER,
    status TEXT NOT NULL DEFAULT 'active',
    plan JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, plan_date, status)
);

-- ユーザーごとのAIコーチ設定
CREATE TABLE user_preferences (
    user_id INTEGER PRIMARY KEY REFERENCES users(id),
    preferred_equipment JSONB NOT NULL DEFAULT '[]'::jsonb,
    avoided_equipment JSONB NOT NULL DEFAULT '[]'::jsonb,
    training_environment TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 初期ユーザー作成
INSERT INTO users (username) VALUES ('Mitsuki');

-- ==========================================
-- ※exercises(種目)のデータは、アプリ起動時にGoサーバーが tmpkin_jp.json を読み込んで自動的に挿入します。
-- そのため、ここにはダミーデータは記述しません。
-- ==========================================
