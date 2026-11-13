import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { autoRetryDelayMs, useSaveAutoRetry } from "./useSaveAutoRetry";

const delays = [2_000, 4_000, 8_000, 16_000, 30_000];

function advance(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

describe("useSaveAutoRetry", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("再送の間隔を広げ、上限に達したら止める", () => {
    expect(delays.map((_, attempt) => autoRetryDelayMs(attempt))).toEqual(delays);
    expect(autoRetryDelayMs(delays.length)).toBeNull();

    const retry = vi.fn();
    const { result } = renderHook(() => useSaveAutoRetry({ locked: true, failed: true, online: true, retry }));

    delays.forEach((delayMs, index) => {
      expect(result.current).toEqual({ status: "scheduled", delayMs });
      advance(delayMs - 1);
      expect(retry).toHaveBeenCalledTimes(index);
      advance(1);
      expect(retry).toHaveBeenCalledTimes(index + 1);
    });

    expect(result.current).toEqual({ status: "exhausted" });
    advance(120_000);
    expect(retry).toHaveBeenCalledTimes(delays.length);
  });

  it("送信中と、入力の見直しで直せる失敗では再送しない", () => {
    const retry = vi.fn();
    const { result, rerender } = renderHook(
      (props: { locked: boolean; failed: boolean }) => useSaveAutoRetry({ ...props, online: true, retry }),
      { initialProps: { locked: true, failed: false } },
    );
    expect(result.current).toEqual({ status: "idle" });

    rerender({ locked: false, failed: true });
    expect(result.current).toEqual({ status: "idle" });

    advance(120_000);
    expect(retry).not.toHaveBeenCalled();
  });

  it("オフライン中は送らず、回線が戻るとすぐに再送する", () => {
    const retry = vi.fn();
    const { result, rerender } = renderHook(
      (props: { online: boolean }) => useSaveAutoRetry({ locked: true, failed: true, online: props.online, retry }),
      { initialProps: { online: false } },
    );
    expect(result.current).toEqual({ status: "offline" });
    advance(120_000);
    expect(retry).not.toHaveBeenCalled();

    act(() => {
      window.dispatchEvent(new Event("online"));
    });
    rerender({ online: true });
    expect(result.current).toEqual({ status: "scheduled", delayMs: 0 });
    advance(0);
    expect(retry).toHaveBeenCalledTimes(1);

    // すぐの再送も失敗したら、最初の間隔から数え直す。
    expect(result.current).toEqual({ status: "scheduled", delayMs: delays[0] });
  });

  it("保存に成功して固定が解けたら、次の失敗は最初の間隔から再送する", () => {
    const retry = vi.fn();
    const { result, rerender } = renderHook(
      (props: { locked: boolean; failed: boolean }) => useSaveAutoRetry({ ...props, online: true, retry }),
      { initialProps: { locked: true, failed: true } },
    );
    advance(delays[0]);
    expect(result.current).toEqual({ status: "scheduled", delayMs: delays[1] });

    rerender({ locked: false, failed: false });
    expect(result.current).toEqual({ status: "idle" });

    rerender({ locked: true, failed: true });
    expect(result.current).toEqual({ status: "scheduled", delayMs: delays[0] });
  });

  it("利用者が先に再試行したら、予約済みの再送を取り消す", () => {
    const retry = vi.fn();
    const { rerender } = renderHook(
      (props: { failed: boolean }) => useSaveAutoRetry({ locked: true, failed: props.failed, online: true, retry }),
      { initialProps: { failed: true } },
    );
    advance(delays[0] - 1);

    rerender({ failed: false });
    advance(120_000);
    expect(retry).not.toHaveBeenCalled();
  });
});
