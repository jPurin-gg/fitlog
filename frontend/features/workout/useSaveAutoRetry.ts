import React from "react";

// 自動再送までの待ち時間。使い切ったあとは手動の再試行か、回線の復帰を待つ。
const AUTO_RETRY_DELAYS_MS = [2_000, 4_000, 8_000, 16_000, 30_000];

export function autoRetryDelayMs(attempt: number): number | null {
  return AUTO_RETRY_DELAYS_MS[attempt] ?? null;
}

export type SaveAutoRetryState =
  | { status: "idle" }
  | { status: "offline" }
  | { status: "scheduled"; delayMs: number }
  | { status: "exhausted" };

interface SaveAutoRetryOptions {
  // 保存済みか判定できないセットがあり、同じ冪等キーでの再送が必要。
  locked: boolean;
  // 直前の送信が失敗して止まっている（送信中ではない）。
  failed: boolean;
  online: boolean;
  retry: () => void;
}

const initialProgress = { attempt: 0, immediate: false };

// 保存済みか判定できないセットを、間隔を広げながら自動で再送する。
// オフライン中は送らず、回線が戻ったら待たずに再送する。
export function useSaveAutoRetry({ locked, failed, online, retry }: SaveAutoRetryOptions): SaveAutoRetryState {
  const [progress, setProgress] = React.useState(initialProgress);
  const retryRef = React.useRef(retry);
  React.useEffect(() => {
    retryRef.current = retry;
  });

  // 保存に成功するなどして固定が解けたら、次の失敗に備えて数え直す。
  React.useEffect(() => {
    if (!locked) setProgress(initialProgress);
  }, [locked]);

  // 回線が戻ったら待ち時間を置かずに再送し、間隔も最初から数え直す。
  React.useEffect(() => {
    if (!locked) return;
    const resumeNow = () => setProgress({ attempt: 0, immediate: true });
    window.addEventListener("online", resumeNow);
    return () => window.removeEventListener("online", resumeNow);
  }, [locked]);

  const waiting = locked && failed;
  const delayMs = !waiting || !online ? null : progress.immediate ? 0 : autoRetryDelayMs(progress.attempt);

  React.useEffect(() => {
    if (delayMs === null) return;
    const timer = setTimeout(() => {
      setProgress(current => ({
        attempt: current.immediate ? current.attempt : current.attempt + 1,
        immediate: false,
      }));
      retryRef.current();
    }, delayMs);
    return () => clearTimeout(timer);
  }, [delayMs, progress]);

  if (!waiting) return { status: "idle" };
  if (!online) return { status: "offline" };
  if (delayMs === null) return { status: "exhausted" };
  return { status: "scheduled", delayMs };
}
