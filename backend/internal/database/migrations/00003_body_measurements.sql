CREATE TABLE IF NOT EXISTS body_measurements (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    measured_on DATE NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    body_fat_percent DOUBLE PRECISION,
    resting_heart_rate INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, measured_on)
);

CREATE INDEX IF NOT EXISTS body_measurements_user_measured_on_idx
    ON body_measurements (user_id, measured_on DESC);
