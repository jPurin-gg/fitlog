import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { CalendarPlanEditor, CalendarWorkoutEditor } from "./CalendarPage";

vi.mock("@/features/exercises/components/ExerciseSelectorModal", () => ({
  ExerciseSelectorModal: ({ onSelect }: { onSelect: (exercise: { id: string; name: string }) => void }) => (
    <button type="button" onClick={() => onSelect({ id: "squat", name: "スクワット" })}>スクワットを選ぶ</button>
  ),
}));

const API = "http://localhost:3000";
let workoutBodies: unknown[] = [];
let planBodies: unknown[] = [];
const server = setupServer(
  http.get(`${API}/api/workouts/by-date/:date`, () => HttpResponse.json({
    workout_id: 5,
    date: "2026-11-10",
    title: "胸の日",
    sets: [{ id: 1, exercise_id: "bench", exercise_name: "ベンチプレス", weight: 60, reps: 10, set_order: 1, feeling: "" }],
  })),
  http.put(`${API}/api/workouts/by-date/:date`, async ({ request }) => {
    workoutBodies.push(await request.json());
    return HttpResponse.json({});
  }),
  http.get(`${API}/api/workout-plans/:date`, () => HttpResponse.json({
    plan_date: "2026-11-20",
    status: "draft",
    plan: {
      workout_title: "脚の日",
      target: "脚",
      estimated_duration_min: 45,
      coach_note: "",
      exercises: [{ exercise_id: "squat", name: "スクワット", planned_sets: 3, target_weight: 80, target_reps: 8 }],
    },
  })),
  http.put(`${API}/api/workout-plans/:date`, async ({ request }) => {
    planBodies.push(await request.json());
    return HttpResponse.json({});
  }),
);
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  workoutBodies = [];
  planBodies = [];
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it("記録の数値欄は空にでき、空のまま保存すると該当欄にエラーを出して送信しない", async () => {
  const onSaved = vi.fn();
  render(<CalendarWorkoutEditor date="2026-11-10" onClose={() => {}} onSaved={onSaved} />);
  const weight = await screen.findByLabelText("重量 kg");
  const reps = screen.getByLabelText("回数");
  expect(weight).toHaveAttribute("inputmode", "decimal");
  expect(reps).toHaveAttribute("inputmode", "numeric");

  await userEvent.clear(weight);
  expect(weight).toHaveValue("");
  await userEvent.click(screen.getByRole("button", { name: "保存" }));

  expect(weight).toHaveAttribute("aria-invalid", "true");
  expect(weight).toHaveAccessibleDescription("重量を入力してください（自重は0）。");
  expect(reps).not.toHaveAttribute("aria-invalid");
  expect(workoutBodies).toHaveLength(0);
  expect(onSaved).not.toHaveBeenCalled();

  await userEvent.type(weight, "62.5");
  expect(weight).not.toHaveAttribute("aria-invalid");
  await userEvent.type(reps, "{Enter}");
  await waitFor(() => expect(onSaved).toHaveBeenCalled());
  expect(workoutBodies).toEqual([{
    title: "胸の日",
    sets: [{ exercise_id: "bench", weight: 62.5, reps: 10, set_order: 1, feeling: "" }],
  }]);
});

it("追加したセットの重量は0ではなく空欄から入力する", async () => {
  render(<CalendarWorkoutEditor date="2026-11-10" onClose={() => {}} onSaved={() => {}} />);
  await screen.findByLabelText("重量 kg");
  await userEvent.click(screen.getByRole("button", { name: "種目" }));
  await userEvent.click(screen.getByRole("button", { name: "スクワットを選ぶ" }));
  const weights = screen.getAllByLabelText("重量 kg");
  expect(weights).toHaveLength(2);
  expect(weights[1]).toHaveValue("");
  await userEvent.type(weights[1], "60");
  expect(weights[1]).toHaveValue("60");
});

it("予定の空欄は既定値（45・3・10）に置き換えず、欄ごとにエラーにする", async () => {
  const onSaved = vi.fn();
  render(<CalendarPlanEditor date="2026-11-20" onClose={() => {}} onSaved={onSaved} />);
  const duration = await screen.findByLabelText("目安分");
  const sets = screen.getByLabelText("セット");
  const weight = screen.getByLabelText("重量 kg");
  const reps = screen.getByLabelText("回数");
  expect(duration).toHaveAttribute("inputmode", "numeric");
  expect(sets).toHaveAttribute("inputmode", "numeric");
  expect(weight).toHaveAttribute("inputmode", "decimal");

  for (const input of [duration, sets, reps]) await userEvent.clear(input);
  await userEvent.click(screen.getByRole("button", { name: "予定を保存" }));

  expect(duration).toHaveAccessibleDescription("目安分を入力してください。");
  expect(sets).toHaveAccessibleDescription("セット数を入力してください。");
  expect(reps).toHaveAccessibleDescription("回数を入力してください。");
  expect(weight).not.toHaveAttribute("aria-invalid");
  expect(screen.getByText(/入力内容を確認してください/)).toBeInTheDocument();
  expect(duration).toHaveValue("");
  expect(planBodies).toHaveLength(0);

  await userEvent.type(duration, "50");
  await userEvent.type(sets, "4");
  await userEvent.type(reps, "6{Enter}");
  await waitFor(() => expect(onSaved).toHaveBeenCalled());
  expect(planBodies).toEqual([expect.objectContaining({
    estimated_duration_min: 50,
    exercises: [{ exercise_id: "squat", name: "スクワット", planned_sets: 4, target_weight: 80, target_reps: 6 }],
  })]);
});
