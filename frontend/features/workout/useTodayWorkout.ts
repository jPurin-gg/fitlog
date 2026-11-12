"use client";

import React from "react";
import { ApiError } from "@/lib/api";
import { formatLocalDate } from "@/lib/fitlog";
import { getWorkoutByDate, getWorkoutDetail } from "./api";
import type { WorkoutDetail } from "./types";

// 読み取りだけで当日の状態を確認する。完了済み記録の再開は開始APIに任せる。
export function useTodayWorkout() {
  const [workout, setWorkout] = React.useState<WorkoutDetail | null>(null);
  const [revision, setRevision] = React.useState(0);
  const refresh = React.useCallback(() => setRevision(value => value + 1), []);

  React.useEffect(() => {
    const controller = new AbortController();
    const load = async () => {
      try {
        const record = await getWorkoutByDate(formatLocalDate(new Date()), controller.signal);
        const detail = await getWorkoutDetail(String(record.workout_id), controller.signal);
        if (!controller.signal.aborted) setWorkout(detail);
      } catch (error) {
        if (controller.signal.aborted) return;
        setWorkout(null);
        if (!(error instanceof ApiError && error.status === 404)) console.error(error);
      }
    };
    void load();
    window.addEventListener("focus", refresh);
    return () => {
      controller.abort();
      window.removeEventListener("focus", refresh);
    };
  }, [refresh, revision]);

  return { workout, refresh };
}
