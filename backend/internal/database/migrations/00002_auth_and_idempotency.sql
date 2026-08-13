DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM users
        GROUP BY username
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot add users username uniqueness: duplicate usernames exist';
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS users_username_unique ON users (username);

ALTER TABLE workout_sets ADD COLUMN IF NOT EXISTS idempotency_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS workout_sets_workout_idempotency_unique
    ON workout_sets (workout_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
