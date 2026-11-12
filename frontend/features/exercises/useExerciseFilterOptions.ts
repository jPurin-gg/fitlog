"use client";

import { useCallback, useEffect, useState } from "react";
import { getExerciseFilterOptions } from "./api";
import type { MuscleCategory } from "./types";

interface FilterOptionsState {
  equipment: string[];
  muscleCategories: MuscleCategory[];
  status: "loading" | "ready" | "error";
}

const LOADING_STATE: FilterOptionsState = { equipment: [], muscleCategories: [], status: "loading" };

// 器具・部位の選択肢はバックエンドが種目カタログと同じ表記で定義している。
// 画面ごとに一覧を持たず、種目辞書・種目セレクター・器具設定のすべてがここから取得する。
export function useExerciseFilterOptions() {
  const [state, setState] = useState<FilterOptionsState>(LOADING_STATE);
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setState(LOADING_STATE);
    getExerciseFilterOptions(controller.signal)
      .then(options => {
        if (controller.signal.aborted) return;
        setState({
          equipment: options.equipment ?? [],
          muscleCategories: options.muscle_categories ?? [],
          status: "ready",
        });
      })
      .catch(error => {
        if (controller.signal.aborted) return;
        console.error("Failed to fetch exercise filter options:", error);
        setState({ ...LOADING_STATE, status: "error" });
      });
    return () => controller.abort();
  }, [reloadToken]);

  const retry = useCallback(() => setReloadToken(token => token + 1), []);

  return { ...state, retry };
}
