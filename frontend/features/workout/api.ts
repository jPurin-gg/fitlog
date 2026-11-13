import { apiFetch } from "@/lib/api";
import type { SetInput, SetPatch, SetRecommendation, WorkoutDetail, WorkoutPlanSession, WorkoutSet, WorkoutSummary } from "./types";

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
  return apiFetch<WorkoutSet>(`/api/workouts/${workoutID}/sets`, {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
    body: JSON.stringify(input),
  });
}

export function updateWorkoutSet(workoutID: number, setID: number, patch: SetPatch) {
  return apiFetch<WorkoutSet>(`/api/workouts/${workoutID}/sets/${setID}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

export function deleteWorkoutSet(workoutID: number, setID: number) {
  return apiFetch<void>(`/api/workouts/${workoutID}/sets/${setID}`, { method: "DELETE" });
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
