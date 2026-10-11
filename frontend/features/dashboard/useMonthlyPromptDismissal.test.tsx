import { act, renderHook, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { useMonthlyPromptDismissal } from "./useMonthlyPromptDismissal";

it("当月の非表示を再マウント後も維持する", async () => {
  const first = renderHook(() => useMonthlyPromptDismissal(1, "2026-10"));
  await waitFor(() => expect(first.result.current.ready).toBe(true));
  expect(first.result.current.dismissed).toBe(false);
  act(() => first.result.current.dismiss());
  first.unmount();
  const second = renderHook(() => useMonthlyPromptDismissal(1, "2026-10"));
  await waitFor(() => expect(second.result.current.ready).toBe(true));
  expect(second.result.current.dismissed).toBe(true);
});

it.each([
  [1, "2026-11"],
  [1, "2027-01"],
  [1, "2027-10"],
  [2, "2026-10"],
])("別のユーザー・年月では抑止しない (%s, %s)", async (userID, month) => {
  const hook = renderHook(({ userID, month }) => useMonthlyPromptDismissal(userID, month), { initialProps: { userID: 1, month: "2026-10" } });
  act(() => hook.result.current.dismiss());
  hook.rerender({ userID, month });
  await waitFor(() => expect(hook.result.current.ready).toBe(true));
  expect(hook.result.current.dismissed).toBe(false);
});

it("12月の非表示を翌年1月には引き継がない", () => {
  window.localStorage.setItem("fitlog:monthly-prompt-dismissed:1", "2026-12");
  const { result } = renderHook(() => useMonthlyPromptDismissal(1, "2027-01"));
  expect(result.current.dismissed).toBe(false);
});

it("不正な保存値は非表示として扱わない", () => {
  window.localStorage.setItem("fitlog:monthly-prompt-dismissed:1", "true");
  const { result } = renderHook(() => useMonthlyPromptDismissal(1, "2026-10"));
  expect(result.current.ready).toBe(true);
  expect(result.current.dismissed).toBe(false);
});

it("ストレージを使えないときも今回の案内は閉じられる", () => {
  vi.spyOn(window.localStorage, "getItem").mockImplementation(() => { throw new Error("unavailable"); });
  vi.spyOn(window.localStorage, "setItem").mockImplementation(() => { throw new Error("unavailable"); });
  const { result } = renderHook(() => useMonthlyPromptDismissal(1, "2026-10"));
  expect(result.current.ready).toBe(true);
  expect(result.current.dismissed).toBe(false);
  act(() => result.current.dismiss());
  expect(result.current.dismissed).toBe(true);
});

it("別タブの変更と保存内容のクリアを反映する", () => {
  const { result } = renderHook(() => useMonthlyPromptDismissal(1, "2026-10"));
  act(() => window.dispatchEvent(new StorageEvent("storage", { key: "fitlog:monthly-prompt-dismissed:1", newValue: "2026-10" })));
  expect(result.current.dismissed).toBe(true);
  act(() => window.dispatchEvent(new StorageEvent("storage", { key: "fitlog:monthly-prompt-dismissed:2", newValue: null })));
  expect(result.current.dismissed).toBe(true);
  act(() => window.dispatchEvent(new StorageEvent("storage", { key: null })));
  expect(result.current.dismissed).toBe(false);
});
