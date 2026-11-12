import { ApiError, apiFetch } from "@/lib/api";
import { emptyRecordedProgress, progressFromSets, type RecordedProgress } from "./recordedProgress";
import type { DatedWorkout, SetInput, SetRecommendation, TodayWorkoutStatus, WorkoutDetail, WorkoutPlanSession, WorkoutSummary } from "./types";

export function getTargetSets(exerciseID: string) {
  return apiFetch<{ target_sets: number }>(`/api/exercises/${encodeURIComponent(exerciseID)}/settings`);
}

export function updateTargetSets(exerciseID: string, targetSets: number) {
  return apiFetch<{ target_sets: number }>(`/api/exercises/${encodeURIComponent(exerciseID)}/settings`, {
    method: "PUT",
    body: JSON.stringify({ target_sets: targetSets }),
  });
}

export function startWorkoutPlan(date: string, input: { use_ai: boolean; exercise_id?: string }, signal?: AbortSignal) {
  return apiFetch<WorkoutPlanSession>(`/api/workout-plans/${date}/start`, {
    method: "POST",
    body: JSON.stringify(input),
    signal,
  });
}

export function getSetRecommendation(workoutID: number, setID: number, signal: AbortSignal) {
  return apiFetch<SetRecommendation>(`/api/workouts/${workoutID}/sets/${setID}/recommendation`, {
    method: "POST",
    signal,
  });
}

export function recordWorkoutSet(workoutID: number, idempotencyKey: string, input: SetInput) {
  return apiFetch<{ id: number }>(`/api/workouts/${workoutID}/sets`, {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
    body: JSON.stringify(input),
  });
}

export function getSummaryComment(workoutID: number, signal: AbortSignal) {
  return apiFetch<{ comment: string; replayed: boolean }>(`/api/workouts/${workoutID}/summary-comment`, {
    method: "POST",
    signal,
  });
}

export function finishWorkoutSession(workoutID: number) {
  return apiFetch<{ summary: WorkoutSummary }>(`/api/workouts/${workoutID}/finish`, { method: "POST" });
}

export function getWorkoutDetail(workoutID: string | number, signal?: AbortSignal) {
  return apiFetch<WorkoutDetail>(`/api/workouts/${encodeURIComponent(String(workoutID))}`, { signal });
}

export function getWorkoutByDate(date: string, signal?: AbortSignal) {
  return apiFetch<DatedWorkout>(`/api/workouts/by-date/${date}`, { signal });
}

// その日の最新のワークアウトが進行中か完了済みかを、参照APIだけで判定する。
export async function getTodayWorkoutStatus(date: string, signal?: AbortSignal): Promise<TodayWorkoutStatus> {
  let workout: DatedWorkout;
  try {
    workout = await getWorkoutByDate(date, signal);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return { state: "none" };
    throw error;
  }
  const detail = await getWorkoutDetail(workout.workout_id, signal);
  if (detail.status !== "active") return { state: "completed", workoutID: workout.workout_id };
  return { state: "active", workoutID: workout.workout_id, setCount: workout.sets?.length ?? 0 };
}

// 開始APIが返したワークアウトの記録済みセットを復元する。
export async function getRecordedProgress(workoutID: number, date: string, signal?: AbortSignal): Promise<RecordedProgress> {
  try {
    const workout = await getWorkoutByDate(date, signal);
    if (workout.workout_id === workoutID) {
      if (!Array.isArray(workout.sets)) throw new Error("記録済みセットの内訳を確認できませんでした。");
      return progressFromSets(workout.sets);
    }
  } catch (error) {
    if (!(error instanceof ApiError && error.status === 404)) throw error;
  }
  // 日付指定で返るのはその日の最新の1件だけ。別の記録や404ならID指定で未記録かを確認する。
  // 集計の件数だけでは最大セット番号や直近の種目を復元できないため、記録がある場合は開始を中断する。
  const detail = await getWorkoutDetail(workoutID, signal);
  if (detail.summary?.total_sets === 0 && Array.isArray(detail.summary.exercises) && detail.summary.exercises.length === 0) {
    return emptyRecordedProgress;
  }
  throw new Error("記録済みセットの内訳を確認できませんでした。");
}
