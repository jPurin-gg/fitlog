-- 日次予定は「同じ日に active が高々1件」だけを保証する。
-- 以前の UNIQUE (user_id, plan_date, status) は completed も1日1件に制限していたため、
-- 同じ日の2件目を completed にする更新（終了API、前日分の一括更新）が一意制約違反で失敗していた。
ALTER TABLE workout_plans
    DROP CONSTRAINT IF EXISTS workout_plans_user_id_plan_date_status_key;

-- 終了APIはトランザクション全体がロールバックするため、2件目のワークアウトも
-- 未終了のまま残りうる。同日 completed と併存する古い記録はセットを残して終了する。
-- アプリのタイムゾーンはSQL移行には渡されないため、UTCの前日より前だけを対象にし、
-- 現在記録中のワークアウトは閉じない。残る前日分は通常の期限切れ処理が補完する。
UPDATE workouts AS stuck
SET ended_at = COALESCE(
    (SELECT MAX(created_at) FROM workout_sets WHERE workout_id = stuck.id), stuck.started_at
)
FROM workout_plans AS active
WHERE stuck.id = active.workout_id
  AND stuck.user_id = active.user_id
  AND stuck.ended_at IS NULL
  AND active.status = 'active'
  AND active.plan_date < (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::date - 1
  AND EXISTS (
      SELECT 1 FROM workout_plans AS completed
      WHERE completed.user_id = active.user_id
        AND completed.plan_date = active.plan_date
        AND completed.status = 'completed'
  );

-- 終了済みワークアウトに紐づく予定を completed にそろえる。
-- 前日分の一括更新の途中で失敗し、予定だけ active に残った場合も修復する。
UPDATE workout_plans AS stuck
SET status = 'completed', updated_at = CURRENT_TIMESTAMP
FROM workouts AS ended
WHERE stuck.status = 'active'
  AND stuck.workout_id = ended.id
  AND ended.ended_at IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS workout_plans_user_date_active_unique
    ON workout_plans (user_id, plan_date)
    WHERE status = 'active';
