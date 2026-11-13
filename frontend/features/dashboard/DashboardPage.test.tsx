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
vi.mock("@/features/exercises/components/AlternativeCoachModal", () => ({
  AlternativeCoachModal: ({ onReplace }: { onReplace: (exercise: { id: string; name: string }) => void }) => (
    <button onClick={() => onReplace({ id: "incline_press", name: "インクラインプレス" })}>代替種目を選ぶ</button>
  ),
}));

const emptyToday = { date: "2026-10-01", timezone: "Asia/Tokyo", rest_day: false, plan: null, workout: null };
const savedPlan = {
  id: 7,
  plan_date: "2026-10-02",
  status: "active",
  ai_status: "not_requested",
  plan: {
    workout_title: "カレンダーで編集した背中の日",
    target: "背中",
    estimated_duration_min: 40,
    coach_note: "丁寧に",
    exercises: [
      { exercise_id: "row", name: "ベントオーバーロウ", planned_sets: 4, target_weight: 55, target_reps: 8, last_max_weight: 60 },
      { exercise_id: "pulldown", name: "ラットプルダウン", planned_sets: 3, target_weight: 40, target_reps: 12 },
    ],
  },
};

let monthlyCalls = 0;
const server = setupServer(
  http.get("http://localhost:3000/api/today", () => HttpResponse.json(emptyToday)),
  http.get("http://localhost:3000/api/dashboard", () => HttpResponse.json({ recent_workouts: [] })),
  http.get("http://localhost:3000/api/preferences", () => HttpResponse.json({})),
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
  server.use(
    http.get("http://localhost:3000/api/monthly-plans/:month", () => {
      monthlyCalls++;
      return HttpResponse.json({ recommended_days: [], weekly_routine: [] });
    }),
    http.get("http://localhost:3000/api/today", () => HttpResponse.json({ ...emptyToday, rest_day: true })),
  );
  show();
  await screen.findByText("休息日");
  await waitFor(() => expect(monthlyCalls).toBe(1));
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
  expect(await screen.findByRole("link", { name: "プランなしで記録を始める" })).toHaveAttribute("href", "/workout");
});

it("今日のメニューは保存済みの日次予定を種目・セット・重量・回数つきで表示する", async () => {
  // 月間プランなし（404）＋日次予定あり。ブラウザは10/1でもサーバーの今日（10/2）を表示する。
  server.use(http.get("http://localhost:3000/api/today", () => HttpResponse.json({ ...emptyToday, date: "2026-10-02", plan: savedPlan })));
  show();
  expect(await screen.findByText("カレンダーで編集した背中の日")).toBeInTheDocument();
  expect(screen.getByText("ベントオーバーロウ")).toBeInTheDocument();
  expect(screen.getByText("4セット / 55kg × 8回")).toBeInTheDocument();
  expect(screen.getByText("ラットプルダウン")).toBeInTheDocument();
  expect(screen.getByText("3セット / 40kg × 12回")).toBeInTheDocument();
  expect(screen.getByText("10月2日金曜日")).toBeInTheDocument();
  expect(screen.queryByText("月間プランは任意です")).not.toBeInTheDocument();
  expect(screen.queryByText("休息日")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "筋トレを開始" })).toHaveAttribute("href", "/workout");
});

it("サーバーが休息日と判定した日だけ休息日を表示する", async () => {
  server.use(http.get("http://localhost:3000/api/today", () => HttpResponse.json({ ...emptyToday, rest_day: true })));
  show();
  expect(await screen.findByText("休息日")).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "筋トレを開始" })).not.toBeInTheDocument();
});

it("種目の変更はサーバーの日付の日次予定として保存し、表示を取り直す", async () => {
  let current: { plan: object } & Record<string, unknown> = { ...emptyToday, date: "2026-10-02", plan: { ...savedPlan, status: "draft" } };
  let savedPath = "";
  let savedBody: { exercises?: unknown[] } | undefined;
  server.use(
    http.get("http://localhost:3000/api/today", () => HttpResponse.json(current)),
    http.put("http://localhost:3000/api/workout-plans/:date", async ({ request, params }) => {
      savedPath = String(params.date);
      savedBody = await request.json() as { exercises?: unknown[] };
      current = { ...current, plan: { ...savedPlan, status: "active", plan: { ...savedPlan.plan, exercises: savedBody.exercises } } };
      return HttpResponse.json(current.plan);
    }),
  );
  show();
  await screen.findByText("月間プランより");
  await userEvent.click(screen.getAllByRole("button", { name: /種目を変更/ })[0]);
  await userEvent.click(await screen.findByRole("button", { name: "代替種目を選ぶ" }));

  expect(await screen.findByText("インクラインプレス")).toBeInTheDocument();
  expect(savedPath).toBe("2026-10-02");
  expect(savedBody?.exercises).toEqual([
    { exercise_id: "incline_press", name: "インクラインプレス", planned_sets: 4, target_weight: 0, target_reps: 8 },
    { exercise_id: "pulldown", name: "ラットプルダウン", planned_sets: 3, target_weight: 40, target_reps: 12 },
  ]);
  expect(screen.getByText("今日の予定")).toBeInTheDocument();
  expect(screen.getByText("4セット / 8回")).toBeInTheDocument();
});
