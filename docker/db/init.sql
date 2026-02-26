-- ユーザー
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 種目 (ベンチプレス, スクワットなど)
CREATE TABLE exercises (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    target_muscle TEXT,
    description TEXT
);

-- ワークアウトセッション
CREATE TABLE workouts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP WITH TIME ZONE,
    notes TEXT
);

-- セットごとの記録
CREATE TABLE workout_sets (
    id SERIAL PRIMARY KEY,
    workout_id INTEGER REFERENCES workouts(id),
    exercise_id INTEGER REFERENCES exercises(id),
    weight FLOAT NOT NULL,
    reps INTEGER NOT NULL,
    set_order INTEGER NOT NULL,
    feeling TEXT, -- セット後の感想 (例: "軽い", "きつい", "左肩に違和感")
    is_pr BOOLEAN DEFAULT FALSE, -- 今回のセットが自己ベスト(PR)更新かどうか
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 初期データ
INSERT INTO exercises (name, target_muscle) VALUES 
('ベンチプレス', '胸'),
('スクワット', '脚'),
('デッドリフト', '背中'),
('ショルダープレス', '肩'),
('懸垂', '背中');
