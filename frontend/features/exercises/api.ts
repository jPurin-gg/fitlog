import { apiFetch } from "@/lib/api";
import type { Exercise, ExerciseFilterOptions, ExercisePage } from "./types";

export const EXERCISE_PAGE_SIZE = 50;

export interface ExerciseSearchFilters {
  name: string;
  // 複数指定した場合は、いずれかに一致する種目が対象になる。
  muscles: string[];
  equipment: string[];
  level: string;
}

export function searchExercises(filters: ExerciseSearchFilters, offset: number, signal: AbortSignal) {
  const params = new URLSearchParams();
  if (filters.name) params.set("name", filters.name);
  if (filters.muscles.length > 0) params.set("muscle", filters.muscles.join(","));
  if (filters.equipment.length > 0) params.set("equipment", filters.equipment.join(","));
  if (filters.level) params.set("level", filters.level);
  params.set("limit", String(EXERCISE_PAGE_SIZE));
  params.set("offset", String(offset));
  return apiFetch<ExercisePage>(`/api/exercises?${params.toString()}`, { signal });
}

export function getExercise(exerciseID: string, signal: AbortSignal) {
  return apiFetch<Exercise>(`/api/exercises/${encodeURIComponent(exerciseID)}`, { signal });
}

export function getExerciseFilterOptions(signal: AbortSignal) {
  return apiFetch<ExerciseFilterOptions>("/api/exercises/filters", { signal });
}
