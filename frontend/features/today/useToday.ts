"use client";

import React from "react";
import { getToday, type Today } from "./api";

/**
 * サーバーが返す今日（日付・タイムゾーン・予定）を読み込む。
 * 画面復帰時にも取り直し、開いたままの日付またぎに追従する。
 */
export function useToday() {
  const [today, setToday] = React.useState<Today | null>(null);
  const [error, setError] = React.useState<unknown>(null);
  const requestRef = React.useRef(0);
  const abortRef = React.useRef<AbortController | null>(null);

  const refresh = React.useCallback(() => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    const requestID = ++requestRef.current;
    return getToday(controller.signal)
      .then(data => {
        if (controller.signal.aborted || requestID !== requestRef.current) return;
        setToday(data);
        setError(null);
      })
      .catch(err => {
        if (controller.signal.aborted || requestID !== requestRef.current) return;
        console.error("Failed to fetch today:", err);
        setError(err);
      });
  }, []);

  React.useEffect(() => {
    void refresh();
    const onFocus = () => { void refresh(); };
    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") void refresh();
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisibilityChange);
      abortRef.current?.abort();
    };
  }, [refresh]);

  return { today, error, refresh };
}
