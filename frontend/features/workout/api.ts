import { apiFetch } from "@/lib/api";
import type { SetInput, SetRecommendation, WorkoutDetail, WorkoutPlanSession, WorkoutSummary } from "./types";

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

export function getWorkoutDetail(workoutID: string) {
  return apiFetch<WorkoutDetail>(`/api/workouts/${encodeURIComponent(workoutID)}`);
}
