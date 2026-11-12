import React from "react";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { AISettingsProvider } from "@/components/AISettings";
import { AuthGate } from "@/components/AuthGate";
import WorkoutPage from "./WorkoutPage";
import type { SavedWorkoutSet } from "./types";

const origin = "http://localhost:3000";
const server = setupServer();
let saved: SavedWorkoutSet[];
let status: string;
let starts: number;
const plan = {
  id: 10, workout_id: 42, plan_date: "2026-11-13", status: "active", ai_status: "not_requested",
  plan: { workout_title: "今日のプラン", target: "胸", estimated_duration_min: 30, coach_note: "",
    exercises: [{ exercise_id: "bench", name: "ベンチプレス", planned_sets: 3, target_weight: 60, target_reps: 8 }] },
};

function detail() {
  return { id: 42, status, summary: {
    total_sets: saved.length, total_reps: 0, total_volume: 0, duration_min: 1, pr_count: 0,
    exercises: [...new Set(saved.map(set => set.exercise_id))].map(id => {
      const sets = saved.filter(set => set.exercise_id === id);
      return { exercise_id: id, name: sets[0].exercise_name, sets: sets.length,
        last_set_order: Math.max(...sets.map(set => set.set_order)), total_reps: 0, best_weight: 60, total_volume: 0 };
    }),
  } };
}

function show() {
  return render(<AISettingsProvider userID={1}><WorkoutPage /><a href="/exercises">辞書へ</a></AISettingsProvider>);
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterAll(() => server.close());
afterEach(() => server.resetHandlers());
beforeEach(() => {
  starts = 0;
  status = "active";
  saved = [1, 2].map(order => ({ id: order, exercise_id: "bench", exercise_name: "ベンチプレス", set_order: order, weight: 60, reps: 8, feeling: "余裕あり" }));
  vi.spyOn(window, "confirm").mockReturnValue(false);
  vi.spyOn(console, "error").mockImplementation(() => {});
  server.use(
    http.get(`${origin}/api/auth/me`, () => HttpResponse.json({ id: 1, nickname: "テスト" })),
    http.post(`${origin}/api/auth/login`, () => HttpResponse.json({ id: 1, nickname: "テスト" })),
    http.get(`${origin}/api/exercises/:id/settings`, () => HttpResponse.json({ target_sets: 3 })),
    http.get(`${origin}/api/workouts/by-date/:date`, () => HttpResponse.json({ workout_id: 42, sets: saved })),
    http.get(`${origin}/api/workouts/42`, () => HttpResponse.json(detail())),
    http.post(`${origin}/api/workout-plans/:date/start`, () => {
      starts++;
      const resumed = status === "completed";
      status = "active";
      return HttpResponse.json({ ...plan, resumed });
    }),
    http.post(`${origin}/api/workouts/42/sets`, async ({ request }) => {
      const set = await request.json() as SavedWorkoutSet;
      expect(saved.some(previous => previous.exercise_id === set.exercise_id && previous.set_order === set.set_order)).toBe(false);
      const id = 100 + saved.length;
      saved.push({ ...set, id, exercise_name: set.exercise_id === "bench" ? "ベンチプレス" : "スクワット" });
      return HttpResponse.json({ id });
    }),
  );
});

it("保存直後に画面を破棄して戻っても3セット目と一覧を復元し、重複せず追記する", async () => {
  saved = [];
  const user = userEvent.setup();
  const first = show();
  await user.click(screen.getByRole("button", { name: "ワークアウトを開始" }));
  await user.click(await screen.findByRole("button", { name: "セットを保存" }));
  await user.click(await screen.findByRole("button", { name: "同じ内容で次へ" }));
  await user.click(screen.getByRole("button", { name: "セットを保存" }));
  await screen.findByText(/セットは保存済みです/);
  first.unmount(); // リロード・ルート移動でReactのstate/refを失う状況。
  show();
  await user.click(await screen.findByRole("button", { name: "ワークアウトを再開" }));
  await screen.findByText("セット 3 / 3");
  expect(within(screen.getByRole("region", { name: "今日ここまでの記録" })).getAllByRole("listitem")).toHaveLength(2);
  await user.click(screen.getByRole("button", { name: "セットを保存" }));
  await screen.findByText(/セットは保存済みです/);
  expect(saved.map(set => set.set_order)).toEqual([1, 2, 3]);
  expect(within(screen.getByRole("region", { name: "今日ここまでの記録" })).getAllByRole("listitem")).toHaveLength(3);
});

it("予定外の最後の種目も復元し、件数ではなくlast_set_orderから種目別の番号を決める", async () => {
  saved[1].set_order = 7;
  saved.push({ id: 3, exercise_id: "squat", exercise_name: "スクワット", set_order: 4, weight: 80, reps: 5, feeling: "" });
  const user = userEvent.setup();
  show();
  await user.click(await screen.findByRole("button", { name: "ワークアウトを再開" }));
  await screen.findByText("セット 5 / 3");
  expect(screen.getByRole("heading", { name: "スクワット" })).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: /ベンチプレス 3セット/ }));
  expect(screen.getByText("セット 8 / 3")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "セットを保存" }));
  await screen.findByText(/セットは保存済みです/);
  expect(saved.at(-1)).toMatchObject({ exercise_id: "bench", set_order: 8 });
});

it("一度別の種目へ移って戻った場合も、最後に保存した種目から再開する", async () => {
  saved.push(
    { id: 3, exercise_id: "squat", exercise_name: "スクワット", set_order: 1, weight: 80, reps: 5, feeling: "" },
    { id: 4, exercise_id: "bench", exercise_name: "ベンチプレス", set_order: 7, weight: 60, reps: 8, feeling: "" },
  );
  const user = userEvent.setup();
  show();
  await user.click(await screen.findByRole("button", { name: "ワークアウトを再開" }));
  await screen.findByText("セット 8 / 3");
  expect(screen.getByRole("heading", { name: "ベンチプレス" })).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: /スクワット 3セット/ }));
  expect(screen.getByText("セット 2 / 3")).toBeInTheDocument();
});

it("完了済みの日も開始ボタンを維持し、同じ記録に追記できる", async () => {
  status = "completed";
  const user = userEvent.setup();
  show();
  const resume = await screen.findByRole("button", { name: "ワークアウトを再開して追記" });
  expect(screen.getByRole("link", { name: "今日の記録を見る" })).toHaveAttribute("href", "/workouts/42");
  expect(starts).toBe(0);
  await user.click(resume);
  await screen.findByText("セット 3 / 3");
  await user.click(screen.getByRole("button", { name: "セットを保存" }));
  await screen.findByText(/セットは保存済みです/);
  expect(saved.map(set => set.set_order)).toEqual([1, 2, 3]);
  expect(starts).toBe(1);
});

it("認証画面への差し替え後も再ログインして保存済みの進捗を復元できる", async () => {
  const user = userEvent.setup();
  render(<AuthGate><WorkoutPage /></AuthGate>);
  await user.click(await screen.findByRole("button", { name: "ワークアウトを再開" }));
  await screen.findByText("セット 3 / 3");
  act(() => { window.dispatchEvent(new Event("fitlog:unauthorized")); });
  await screen.findByRole("heading", { name: "Fitlog に入る" });
  await user.type(screen.getByPlaceholderText("例: 筋トレマン"), "テスト");
  await user.type(screen.getByPlaceholderText("パスワード"), "mock-password");
  await user.click(screen.getByRole("button", { name: "ログイン" }));
  await user.click(await screen.findByRole("button", { name: "ワークアウトを再開" }));
  await screen.findByText("セット 3 / 3");
  expect(within(screen.getByRole("region", { name: "今日ここまでの記録" })).getAllByRole("listitem")).toHaveLength(2);
});

it("一覧APIの取得に失敗しても詳細APIのlast_set_orderで安全に再開する", async () => {
  let listReads = 0;
  server.use(
    http.get(`${origin}/api/workouts/by-date/:date`, () => {
      if (++listReads === 1) return HttpResponse.json({ workout_id: 42, sets: saved });
      return HttpResponse.json({ detail: "一覧を取得できません。" }, { status: 503 });
    }),
    http.get(`${origin}/api/workouts/42`, () => {
      const data = detail();
      data.summary.exercises[0].last_set_order = 7;
      return HttpResponse.json(data);
    }),
  );
  const user = userEvent.setup();
  show();
  await user.click(await screen.findByRole("button", { name: "ワークアウトを再開" }));
  await screen.findByText("セット 8 / 3");
  expect(screen.getByRole("alert")).toHaveTextContent("一覧を取得できません。");
  expect(screen.queryByText("保存済みのセットはまだありません。")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "セットを保存" }));
  await screen.findByText(/セットは保存済みです/);
  expect(saved.at(-1)?.set_order).toBe(8);
});

it("日付別APIが別のワークアウトを返してもそのセットを混ぜない", async () => {
  let listReads = 0;
  server.use(http.get(`${origin}/api/workouts/by-date/:date`, () => HttpResponse.json({ workout_id: ++listReads === 1 ? 42 : 99, sets: saved })));
  const user = userEvent.setup();
  show();
  await user.click(await screen.findByRole("button", { name: "ワークアウトを再開" }));
  await screen.findByText("セット 3 / 3");
  expect(screen.getByRole("alert")).toHaveTextContent("当日の記録とワークアウトが一致しません。");
  expect(within(screen.getByRole("region", { name: "今日ここまでの記録" })).queryByRole("listitem")).not.toBeInTheDocument();
});
