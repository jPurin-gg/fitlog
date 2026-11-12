"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, apiErrorMessage, apiFetch } from "@/lib/api";

type QueryState<T> = {
  key: string;
  status: "loading" | "success" | "error";
  data: T | null;
  error: string;
};

export function useApiQuery<T>(
  path: string,
  errorMessage: string,
  { scope = "", allowNotFound = false, delay = 0 }: { scope?: string | number; allowNotFound?: boolean; delay?: number } = {},
) {
  const key = JSON.stringify([scope, path]);
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<QueryState<T>>({ key, status: "loading", data: null, error: "" });

  const retry = useCallback(() => {
    setState({ key, status: "loading", data: null, error: "" });
    setAttempt(previous => previous + 1);
  }, [key]);

  const setData = useCallback((data: T) => {
    setState({ key, status: "success", data, error: "" });
  }, [key]);

  useEffect(() => {
    const controller = new AbortController();
    setState({ key, status: "loading", data: null, error: "" });
    const load = async () => {
      try {
        const data = await apiFetch<T>(path, { signal: controller.signal });
        if (!controller.signal.aborted) setState({ key, status: "success", data, error: "" });
      } catch (error) {
        if (controller.signal.aborted) return;
        if (allowNotFound && error instanceof ApiError && error.status === 404) {
          setState({ key, status: "success", data: null, error: "" });
        } else {
          setState({ key, status: "error", data: null, error: apiErrorMessage(error, errorMessage) });
        }
      }
    };
    const timer = window.setTimeout(load, delay);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [path, key, errorMessage, allowNotFound, delay, attempt]);

  // 対象が変わったレンダーでは、前のユーザー・月・検索条件の値を使わない。
  const current = state.key === key ? state : { status: "loading" as const, data: null, error: "" };
  return { ...current, retry, setData };
}
