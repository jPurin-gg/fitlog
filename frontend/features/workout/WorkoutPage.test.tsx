import React from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AISettingsProvider } from "@/components/AISettings";
import WorkoutPage from "./WorkoutPage";

const API = "http://localhost:3000";
const planSession = {
  id: 1,
  workout_id: 10,
  plan_date: "2026-11-13",
  status: "planned",
  ai_status: "not_requested",
  plan: {
    workout_title: "胸の日",
    target: "胸",
    estimated_duration_min: 45,
    coach_note: "",
    exercises: [{ exercise_id: "bench", name: "ベンチプレス", planned_sets: 3, target_weight: 0, target_reps: 0 }],
  },
};

let targetSetsBodies: unknown[] = [];
let recordedSets: unknown[] = [];
const server = setupServer(
  http.get(`${API}/api/exercises/:id/settings`, () => HttpResponse.json({ target_sets: 3 })),
  http.put(`${API}/api/exercises/:id/settings`, async ({ request }) => {
    const body = await request.json();
    targetSetsBodies.push(body);
    return HttpResponse.json(body);
  }),
  http.post(`${API}/api/workout-plans/:date/start`, () => HttpResponse.json(planSession)),
  http.post(`${API}/api/workouts/:id/sets`, async ({ request }) => {
    recordedSets.push(await request.json());
    return HttpResponse.json({ id: 100 });
  }),
);
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  targetSetsBodies = [];
  recordedSets = [];
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

async function startWorkout() {
  render(<AISettingsProvider userID={1}><WorkoutPage /></AISettingsProvider>);
  await userEvent.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
  await screen.findByText("セット 1 / 3");
}

it("重量・回数に数字キーパッドを指定し、Enterでセットを保存する", async () => {
  await startWorkout();
  const weight = screen.getByLabelText("重量（kg）");
  const reps = screen.getByLabelText("回数");
  expect(weight).toHaveAttribute("inputmode", "decimal");
  expect(reps).toHaveAttribute("inputmode", "numeric");

  await userEvent.type(weight, "62.5{Enter}");
  expect(reps).toHaveFocus();
  expect(recordedSets).toHaveLength(0);

  await userEvent.type(reps, "8{Enter}");
  await screen.findByText(/セットは保存済みです/);
  expect(recordedSets).toEqual([expect.objectContaining({ exercise_id: "bench", weight: 62.5, reps: 8, set_order: 1 })]);
});

it("目標セット数は開いたまま3から6に増やし、確定で保存する", async () => {
  await startWorkout();
  await userEvent.click(screen.getByRole("button", { name: "目標セット数を変更" }));
  const editor = screen.getByRole("group", { name: "目標セット数の編集" });
  const increase = within(editor).getByRole("button", { name: "目標セット数を増やす" });
  await userEvent.click(increase);
  await userEvent.click(increase);
  await userEvent.click(increase);
  expect(within(editor).getByText("6 セット")).toBeInTheDocument();
  expect(targetSetsBodies).toHaveLength(0);

  await userEvent.click(within(editor).getByRole("button", { name: "確定" }));
  await screen.findByText("セット 1 / 6");
  expect(targetSetsBodies).toEqual([{ target_sets: 6 }]);
  expect(screen.queryByRole("group", { name: "目標セット数の編集" })).not.toBeInTheDocument();
});

it("目標セット数の保存失敗を表示し、編集欄を開いたまま再試行できる", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  server.use(http.put(`${API}/api/exercises/:id/settings`, () => HttpResponse.json(
    { status: 500, detail: "保存できませんでした。" },
    { status: 500, headers: { "Content-Type": "application/problem+json" } },
  ), { once: true }));
  await startWorkout();
  await userEvent.click(screen.getByRole("button", { name: "目標セット数を変更" }));
  const editor = screen.getByRole("group", { name: "目標セット数の編集" });
  await userEvent.click(within(editor).getByRole("button", { name: "目標セット数を増やす" }));
  await userEvent.click(within(editor).getByRole("button", { name: "確定" }));

  expect(await within(editor).findByRole("alert")).toHaveTextContent("保存できませんでした。");
  expect(screen.getByText("セット 1 / 3")).toBeInTheDocument();
  expect(within(editor).getByText("4 セット")).toBeInTheDocument();

  await userEvent.click(within(editor).getByRole("button", { name: "もう一度確定" }));
  await screen.findByText("セット 1 / 4");
  expect(targetSetsBodies).toEqual([{ target_sets: 4 }]);
});

it("記録画面の主要な操作は44px四方以上のタップ領域を持つ", async () => {
  await startWorkout();
  await userEvent.click(screen.getByRole("button", { name: "目標セット数を変更" }));
  const editor = screen.getByRole("group", { name: "目標セット数の編集" });
  // jsdomはレイアウトを計算しないため、Tailwindの寸法クラス（h-11/min-h-11 = 2.75rem = 44px）で確認する。
  for (const name of ["目標セット数を減らす", "目標セット数を増やす"]) {
    const button = within(editor).getByRole("button", { name });
    expect(button).toHaveClass("h-11", "w-11");
  }
  for (const button of [
    within(editor).getByRole("button", { name: "確定" }),
    within(editor).getByRole("button", { name: "キャンセル" }),
    screen.getByRole("button", { name: "目標セット数を変更" }),
    screen.getByRole("button", { name: "種目を検索して変更" }),
    screen.getByRole("button", { name: "AIに代替種目を聞く" }),
    screen.getByRole("button", { name: "終了" }),
  ]) {
    expect(button).toHaveClass("min-h-11");
  }
  expect(screen.getByRole("button", { name: "終了" })).toHaveClass("min-w-11");
  for (const button of within(editor).getAllByRole("button")) {
    expect(button.className).not.toMatch(/text-\[(?:[0-9]|10)px\]/);
  }
});

it("キャンセルでは保存せず元の目標セット数に戻す", async () => {
  await startWorkout();
  await userEvent.click(screen.getByRole("button", { name: "目標セット数を変更" }));
  const editor = screen.getByRole("group", { name: "目標セット数の編集" });
  await userEvent.click(within(editor).getByRole("button", { name: "目標セット数を減らす" }));
  await userEvent.click(within(editor).getByRole("button", { name: "キャンセル" }));
  await waitFor(() => expect(screen.queryByRole("group", { name: "目標セット数の編集" })).not.toBeInTheDocument());
  expect(screen.getByText("セット 1 / 3")).toBeInTheDocument();
  expect(targetSetsBodies).toHaveLength(0);
});
