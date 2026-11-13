import { apiFetch } from "@/lib/api";
import type { WorkoutPlanSession } from "@/features/workout/types";

export type TodayPlanSession = Omit<WorkoutPlanSession, "id" | "workout_id"> & {
  id?: number;
  workout_id?: number;
};

export interface TodayWorkout {
  id: number;
  status: "active" | "completed";
  started_at: string;
  ended_at?: string;
}

/** サーバー（APP_TIMEZONE）が決めた「今日」。ブラウザの日付から今日を作らない。 */
export interface Today {
  date: string;
  timezone: string;
  rest_day: boolean;
  plan: TodayPlanSession | null;
  workout: TodayWorkout | null;
}

export function getToday(signal?: AbortSignal) {
  return apiFetch<Today>("/api/today", { signal });
}
