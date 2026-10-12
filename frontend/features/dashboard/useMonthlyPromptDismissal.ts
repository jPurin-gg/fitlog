"use client";

import { useCallback, useEffect, useState } from "react";

export function useMonthlyPromptDismissal(userID: number, month: string) {
  const storageKey = `fitlog:monthly-prompt-dismissed:${userID}`;
  const scope = `${userID}:${month}`;
  const [state, setState] = useState({ scope: "", dismissed: false });

  useEffect(() => {
    let dismissed = false;
    try { dismissed = window.localStorage.getItem(storageKey) === month; } catch { /* 案内は利用できる状態を維持 */ }
    setState({ scope, dismissed });
    const sync = (event: StorageEvent) => {
      if (event.key !== storageKey && event.key !== null) return;
      setState({ scope, dismissed: event.key !== null && event.newValue === month });
    };
    window.addEventListener("storage", sync);
    return () => window.removeEventListener("storage", sync);
  }, [storageKey, scope, month]);

  const dismiss = useCallback(() => {
    setState({ scope, dismissed: true });
    try { window.localStorage.setItem(storageKey, month); } catch { /* 保存不可でも今回の案内は閉じる */ }
  }, [storageKey, scope, month]);

  return { ready: state.scope === scope, dismissed: state.scope === scope && state.dismissed, dismiss };
}
