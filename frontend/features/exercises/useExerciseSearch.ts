"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { apiErrorMessage } from "@/lib/api";
import { searchExercises, type ExerciseSearchFilters } from "./api";
import type { ExerciseSummary } from "./types";

const NAME_DEBOUNCE_MS = 300;
const LOAD_ERROR_MESSAGE = "種目を取得できませんでした。通信状況を確認して、もう一度お試しください。";

type SearchStatus = "loading" | "ready" | "error";

interface SearchState {
  items: ExerciseSummary[];
  total: number;
  hasMore: boolean;
  status: SearchStatus;
  error: string;
  loadingMore: boolean;
  loadMoreError: string;
}

// 1つの検索条件に対する取得状況。条件が変わると controller ごと使い捨てる。
interface ActiveQuery {
  controller: AbortController;
  filters: ExerciseSearchFilters;
  offset: number;
  total: number;
  busy: boolean;
}

const LOADING_STATE: SearchState = {
  items: [],
  total: 0,
  hasMore: false,
  status: "loading",
  error: "",
  loadingMore: false,
  loadMoreError: "",
};

function splitKey(key: string): string[] {
  return key ? key.split(",") : [];
}

// 種目一覧をページ単位で取得する。enabled が false の間は取得しない。
export function useExerciseSearch(filters: ExerciseSearchFilters, enabled = true) {
  const name = filters.name.trim();
  const level = filters.level;
  // 配列は描画のたびに作り直されるため、内容を表す文字列で変化を判定する。
  const muscleKey = filters.muscles.join(",");
  const equipmentKey = filters.equipment.join(",");

  const [debouncedName, setDebouncedName] = useState(name);
  const [state, setState] = useState<SearchState>(LOADING_STATE);
  const [reloadToken, setReloadToken] = useState(0);
  const queryRef = useRef<ActiveQuery | null>(null);

  // 入力中に毎回叩かないよう、種目名だけ 300ms 遅らせて反映する。
  useEffect(() => {
    if (name === debouncedName) return;
    const timer = window.setTimeout(() => setDebouncedName(name), NAME_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [name, debouncedName]);

  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    const query: ActiveQuery = {
      controller,
      filters: { name: debouncedName, muscles: splitKey(muscleKey), equipment: splitKey(equipmentKey), level },
      offset: 0,
      total: 0,
      busy: true,
    };
    queryRef.current = query;
    setState(LOADING_STATE);
    searchExercises(query.filters, 0, controller.signal)
      .then(page => {
        if (controller.signal.aborted) return;
        const items = page.items ?? [];
        query.offset = items.length;
        query.total = page.total ?? items.length;
        query.busy = false;
        setState({ ...LOADING_STATE, items, total: query.total, hasMore: query.offset < query.total, status: "ready" });
      })
      .catch(error => {
        if (controller.signal.aborted) return;
        query.busy = false;
        setState({ ...LOADING_STATE, status: "error", error: apiErrorMessage(error, LOAD_ERROR_MESSAGE) });
      });
    // 条件が変わったら古いリクエストを中断し、遅れて届いた結果も反映しない。
    return () => controller.abort();
  }, [enabled, debouncedName, muscleKey, equipmentKey, level, reloadToken]);

  const loadMore = useCallback(() => {
    const query = queryRef.current;
    if (!query || query.controller.signal.aborted || query.busy || query.offset >= query.total) return;
    query.busy = true;
    setState(previous => ({ ...previous, loadingMore: true, loadMoreError: "" }));
    searchExercises(query.filters, query.offset, query.controller.signal)
      .then(page => {
        if (query.controller.signal.aborted) return;
        const items = page.items ?? [];
        query.offset += items.length;
        // 取得の合間に件数が減っても「さらに表示」が残り続けないようにする。
        query.total = items.length === 0 ? query.offset : page.total ?? query.offset;
        query.busy = false;
        const total = query.total;
        const hasMore = query.offset < query.total;
        setState(previous => {
          // 取得の合間に並びがずれても、同じ種目を二重に表示しない。
          const known = new Set(previous.items.map(item => item.id));
          return {
            ...previous,
            items: [...previous.items, ...items.filter(item => !known.has(item.id))],
            total,
            hasMore,
            loadingMore: false,
          };
        });
      })
      .catch(error => {
        if (query.controller.signal.aborted) return;
        query.busy = false;
        setState(previous => ({ ...previous, loadingMore: false, loadMoreError: apiErrorMessage(error, LOAD_ERROR_MESSAGE) }));
      });
  }, []);

  const reload = useCallback(() => setReloadToken(token => token + 1), []);

  // 入力の反映待ちの間も、古い結果ではなく読み込み中として扱う。
  const status: SearchStatus = enabled && name !== debouncedName ? "loading" : state.status;
  return {
    items: state.items,
    total: state.total,
    hasMore: state.hasMore,
    status,
    error: state.error,
    loadingMore: state.loadingMore,
    loadMoreError: state.loadMoreError,
    loadMore,
    reload,
  };
}
