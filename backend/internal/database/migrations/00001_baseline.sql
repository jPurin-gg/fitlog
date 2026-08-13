CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    password_hash TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
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

CREATE TABLE IF NOT EXISTS user_exercise_stats (
    user_id INTEGER REFERENCES users(id),
    exercise_id TEXT REFERENCES exercises(id),
    weight DOUBLE PRECISION,
    max_weight DOUBLE PRECISION,
    target_sets INTEGER DEFAULT 3,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, exercise_id)
);

CREATE TABLE IF NOT EXISTS workouts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP WITH TIME ZONE,
    notes TEXT,
    summary_comment TEXT
);

CREATE TABLE IF NOT EXISTS workout_sets (
    id SERIAL PRIMARY KEY,
    workout_id INTEGER REFERENCES workouts(id),
    exercise_id TEXT REFERENCES exercises(id),
    weight DOUBLE PRECISION NOT NULL,
    reps INTEGER NOT NULL,
    set_order INTEGER NOT NULL,
    feeling TEXT,
    is_pr BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS monthly_plans (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    plan_month TEXT NOT NULL,
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

CREATE TABLE IF NOT EXISTS workout_plans (
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

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id INTEGER PRIMARY KEY REFERENCES users(id),
    preferred_equipment JSONB NOT NULL DEFAULT '[]'::jsonb,
    avoided_equipment JSONB NOT NULL DEFAULT '[]'::jsonb,
    training_environment TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_favorite_exercises (
    user_id INTEGER REFERENCES users(id),
    exercise_id TEXT REFERENCES exercises(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, exercise_id)
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
ALTER TABLE workouts ADD COLUMN IF NOT EXISTS summary_comment TEXT;
ALTER TABLE monthly_plans ADD COLUMN IF NOT EXISTS rest_days JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE user_exercise_stats ADD COLUMN IF NOT EXISTS target_sets INTEGER DEFAULT 3;
