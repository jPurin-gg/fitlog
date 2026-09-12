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

export function startWorkoutPlan(date: string) {
  return apiFetch<WorkoutPlanSession>(`/api/workout-plans/${date}/start`, { method: "POST" });
}

export function saveManualWorkoutPlan(date: string, exercise: { id: string; name: string }) {
  return apiFetch<WorkoutPlanSession>(`/api/workout-plans/${date}`, {
    method: "PUT",
    body: JSON.stringify({
      workout_title: "フリーワークアウト",
      target: "自分で選んだ種目",
      estimated_duration_min: 15,
      coach_note: "体調に合わせて無理のない重量で進めましょう。",
      exercises: [{
        exercise_id: exercise.id,
        name: exercise.name,
        planned_sets: 3,
        target_weight: 0,
        target_reps: 10,
      }],
    }),
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
