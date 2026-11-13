"use client";

import React from "react";

function subscribe(onChange: () => void) {
  window.addEventListener("online", onChange);
  window.addEventListener("offline", onChange);
  return () => {
    window.removeEventListener("online", onChange);
    window.removeEventListener("offline", onChange);
  };
}

// ブラウザが回線につながっているかを返す。電波が弱いだけの状態はオンラインと報告されるため、
// 応答の有無はリクエスト側の待ち時間で判定する。
export function useOnlineStatus(): boolean {
  return React.useSyncExternalStore(subscribe, () => navigator.onLine, () => true);
}
