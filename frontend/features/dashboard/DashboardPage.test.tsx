import React from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AISettingsProvider } from "@/components/AISettings";
import DashboardPage from "./DashboardPage";

const auth = vi.hoisted(() => ({ user: { id: 1, nickname: "テスト" } }));
vi.mock("@/components/AuthGate", () => ({ useAuth: () => auth, LogoutButton: () => null }));

let monthlyCalls = 0;
const server = setupServer(
  http.get("http://localhost:3000/api/dashboard", () => HttpResponse.json({ recent_workouts: [] })),
  http.get("http://localhost:3000/api/preferences", () => HttpResponse.json({})),
  http.get("http://localhost:3000/api/workouts/by-date/:date", () => HttpResponse.json({ detail: "指定日のワークアウトがありません。" }, { status: 404 })),
  http.get("http://localhost:3000/api/monthly-plans/:month", () => {
    monthlyCalls++;
    return HttpResponse.json({ error: "not found" }, { status: 404 });
  }),
);
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-01T12:00:00Z"));
  auth.user.id = 1;
  monthlyCalls = 0;
});
afterEach(() => { server.resetHandlers(); vi.useRealTimers(); vi.restoreAllMocks(); });
afterAll(() => server.close());

function show() {
  return render(<AISettingsProvider userID={auth.user.id}><DashboardPage /></AISettingsProvider>);
}

it("未作成の月の最初のホーム訪問で案内し、AI OFFのまま記録に進める", async () => {
  show();
  const dialog = await screen.findByRole("dialog", { name: "月間プラン" });
  expect(within(dialog).getByRole("switch")).not.toBeChecked();
  expect(within(dialog).getByRole("button", { name: "決定してプランを作成" })).toBeDisabled();
  expect(within(dialog).getByRole("link", { name: "プランなしで記録を始める" })).toHaveAttribute("href", "/workout");
});

it("今月の非表示は再訪後も効き、手動で開いて閉じても解除しない", async () => {
  const first = show();
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "今月は次回以降表示しない" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  first.unmount();
  const second = show();
  await waitFor(() => expect(monthlyCalls).toBe(2));
  await screen.findByText("まだ完了したワークアウトはありません。");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "月間プランを作成／見直す" }));
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "閉じる" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  second.unmount();
  show();
  await waitFor(() => expect(monthlyCalls).toBe(3));
  await screen.findByText("まだ完了したワークアウトはありません。");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("通常の閉じるは今回だけで、次のホーム訪問では案内する", async () => {
  const first = show();
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "閉じる" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  first.unmount();
  show();
  await screen.findByRole("dialog");
});

it("非表示にしたホームを開いたまま翌月になっても再び案内する", async () => {
  show();
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "今月は次回以降表示しない" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  act(() => {
    vi.setSystemTime(new Date("2026-11-01T12:00:00Z"));
    window.dispatchEvent(new Event("focus"));
  });
  await screen.findByRole("dialog");
  expect(monthlyCalls).toBe(2);
});

it("別ユーザーには今月の非表示を引き継がない", async () => {
  window.localStorage.setItem("fitlog:monthly-prompt-dismissed:1", "2026-10");
  auth.user.id = 2;
  show();
  await screen.findByRole("dialog");
});

it("今月のプランがあるときは自動で案内しない", async () => {
  server.use(http.get("http://localhost:3000/api/monthly-plans/:month", () => HttpResponse.json({ recommended_days: [], weekly_routine: [] })));
  show();
  await screen.findByText("休息日");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "月間プランを作成／見直す" }));
  await screen.findByRole("dialog");
});

it("プランの読み込み失敗を未作成と誤認して案内しない", async () => {
  const error = vi.spyOn(console, "error").mockImplementation(() => {});
  server.use(http.get("http://localhost:3000/api/monthly-plans/:month", () => HttpResponse.json({ error: "unavailable" }, { status: 500 })));
  show();
  await waitFor(() => expect(error).toHaveBeenCalled());
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "プランなしで記録を始める" })).toHaveAttribute("href", "/workout");
});

function useTodayWorkout(status: "active" | "completed", setCount: number) {
  server.use(
    http.get("http://localhost:3000/api/workouts/by-date/:date", () => HttpResponse.json({
      workout_id: 42,
      date: "2026-10-01",
      title: "今日の記録",
      sets: Array.from({ length: setCount }, (_, index) => ({ id: index + 1, exercise_id: "bench_press", set_order: index + 1, weight: 60, reps: 8 })),
    })),
    http.get("http://localhost:3000/api/workouts/42", () => HttpResponse.json({ id: 42, status })),
  );
}

it.each([0, 2])("進行中なら保存済み%dセットの再開を案内し、開始導線を置き換える", async setCount => {
  useTodayWorkout("active", setCount);
  window.localStorage.setItem("fitlog:monthly-prompt-dismissed:1", "2026-10");
  show();
  await screen.findByRole("heading", { name: "記録の途中です" });
  const resumeLinks = screen.getAllByRole("link", { name: "記録を再開" });
  expect(resumeLinks).toHaveLength(2);
  for (const link of resumeLinks) expect(link).toHaveAttribute("href", "/workout");
  expect(screen.getByText(setCount > 0
    ? `今日は${setCount}セットを保存済みです。続きから再開できます。`
    : "今日のワークアウトを開始済みです。続きから再開できます。")).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /記録を始める|筋トレを開始/ })).not.toBeInTheDocument();
});

it("月間プランがあっても進行中なら筋トレ開始を再開に置き換える", async () => {
  useTodayWorkout("active", 2);
  server.use(http.get("http://localhost:3000/api/monthly-plans/:month", () => HttpResponse.json({
    recommended_days: [4],
    weekly_routine: [{ day_name: "木曜メニュー", target: "胸", example_exercises: ["ベンチプレス"] }],
  })));
  show();
  await screen.findByText("木曜メニュー");
  await screen.findByRole("heading", { name: "記録の途中です" });
  expect(screen.getAllByRole("link", { name: "記録を再開" })).toHaveLength(2);
  expect(screen.queryByRole("link", { name: "筋トレを開始" })).not.toBeInTheDocument();
});

it("完了済みならホームの開始導線を今日の結果へのリンクに置き換える", async () => {
  useTodayWorkout("completed", 3);
  window.localStorage.setItem("fitlog:monthly-prompt-dismissed:1", "2026-10");
  show();
  await screen.findByRole("heading", { name: "今日のワークアウトは完了しています" });
  const resultLinks = screen.getAllByRole("link", { name: "今日の結果を見る" });
  expect(resultLinks).toHaveLength(2);
  for (const link of resultLinks) expect(link).toHaveAttribute("href", "/workouts/42");
  expect(screen.queryByRole("link", { name: /記録を始める|筋トレを開始|記録を再開/ })).not.toBeInTheDocument();
});
