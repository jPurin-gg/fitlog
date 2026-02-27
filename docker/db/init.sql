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

INSERT INTO users (username) VALUES ('Mitsuki');

-- Dummy Workouts for February 2026
INSERT INTO workouts (user_id, started_at, ended_at, notes) VALUES 
(1, '2026-02-01 10:00:00+09', '2026-02-01 11:00:00+09', 'Leg Day'),
(1, '2026-02-03 18:00:00+09', '2026-02-03 19:15:00+09', 'Push Day'),
(1, '2026-02-05 07:00:00+09', '2026-02-05 07:45:00+09', 'Pull Day'),
(1, '2026-02-10 19:00:00+09', '2026-02-10 20:30:00+09', 'Leg Day'),
(1, '2026-02-15 08:00:00+09', '2026-02-15 09:10:00+09', 'Push Day'),
(1, '2026-02-20 18:30:00+09', '2026-02-20 19:40:00+09', 'Pull Day'),
(1, '2026-02-24 17:00:00+09', '2026-02-24 18:00:00+09', 'Leg Day'),
(1, '2026-02-25 09:00:00+09', '2026-02-25 09:55:00+09', 'Push Day');

-- Dummy sets for a recent workout (Workout id 8 on Feb 25)
INSERT INTO workout_sets (workout_id, exercise_id, weight, reps, set_order, feeling) VALUES 
(8, 1, 60.0, 10, 1, '軽い'),
(8, 1, 80.0, 8, 2, '余裕あり'),
(8, 1, 85.0, 5, 3, '限界'),
(8, 4, 30.0, 12, 1, '普通'),
(8, 4, 35.0, 10, 2, 'きつい');
