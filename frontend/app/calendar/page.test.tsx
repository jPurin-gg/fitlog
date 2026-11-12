import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";

import { CalendarPlanEditor, CalendarWorkoutEditor } from "@/features/calendar/CalendarPage";

import { expectAccessible } from "@/test/a11y";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it("未来の予定を編集し、画面の値を日次プランとして保存する", async () => {
  let savedBody: Record<string, unknown> | undefined;
  const onSaved = vi.fn();
  server.use(
    http.get(`${apiOrigin}/api/workout-plans/2026-09-20`, () => HttpResponse.json({
      id: 12,
      plan_date: "2026-09-20",
      status: "active",
      plan: {
        workout_title: "胸の日",
        target: "胸",
        estimated_duration_min: 45,
        coach_note: "丁寧に行う",
        exercises: [{
          exercise_id: "bench",
          name: "ベンチプレス",
          planned_sets: 3,
          target_weight: 60,
          target_reps: 8,
        }],
      },
    })),
    http.put(`${apiOrigin}/api/workout-plans/2026-09-20`, async ({ request }) => {
      savedBody = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ id: 12, plan_date: "2026-09-20", status: "active", plan: savedBody });
    }),
  );

  const user = userEvent.setup();
  render(<CalendarPlanEditor date="2026-09-20" onClose={() => undefined} onSaved={onSaved} />);

  const title = await screen.findByLabelText("タイトル");
  await expectAccessible(screen.getByRole("dialog", { name: "予定を編集" }));
  await user.clear(title);
  await user.type(screen.getByLabelText("タイトル"), "上半身の日");
  await user.clear(screen.getByLabelText("目安分"));
  await user.type(screen.getByLabelText("目安分"), "55");
  await user.clear(screen.getByLabelText("重量 kg"));
  await user.type(screen.getByLabelText("重量 kg"), "62.5");
  await user.click(screen.getByRole("button", { name: "予定を保存" }));

  expect(onSaved).toHaveBeenCalledOnce();
  expect(savedBody).toEqual({
    workout_title: "上半身の日",
    target: "胸",
    estimated_duration_min: 55,
    coach_note: "丁寧に行う",
    exercises: [{
      exercise_id: "bench",
      name: "ベンチプレス",
      planned_sets: 3,
      target_weight: 62.5,
      target_reps: 8,
    }],
  });
});

it("過去記録のタイトルとセットを編集し、保存に失敗しても入力を保つ", async () => {
  let attempts = 0;
  let body: unknown;
  const onSaved = vi.fn();
  vi.spyOn(console, "error").mockImplementation(() => {});
  server.use(
    http.get(`${apiOrigin}/api/workouts/by-date/2026-09-01`, () => HttpResponse.json({
      workout_id: 42, title: "胸の日", sets: [
        { exercise_id: "bench", exercise_name: "ベンチプレス", weight: 60, reps: 8, set_order: 1, feeling: "余裕" },
        { exercise_id: "bench", exercise_name: "ベンチプレス", weight: 60, reps: 8, set_order: 2, feeling: "" },
      ],
    })),
    http.put(`${apiOrigin}/api/workouts/by-date/2026-09-01`, async ({ request }) => {
      body = await request.json();
      return ++attempts === 1
        ? HttpResponse.json({ detail: "保存できませんでした。" }, { status: 500 })
        : HttpResponse.json({});
    }),
  );
  render(<CalendarWorkoutEditor date="2026-09-01" onClose={vi.fn()} onSaved={onSaved} />);
  const dialog = screen.getByRole("dialog", { name: "記録を編集" });
  const title = await within(dialog).findByRole("textbox", { name: "タイトル" });
  expect(within(dialog).getByRole("link", { name: "詳細" })).toHaveAttribute("href", "/workouts/42");
  await userEvent.clear(title);
  await userEvent.type(title, "胸の記録");
  await userEvent.click(within(dialog).getAllByRole("button", { name: "セットを削除" })[0]);
  await userEvent.clear(within(dialog).getByRole("spinbutton", { name: "重量 kg" }));
  await userEvent.type(within(dialog).getByRole("spinbutton", { name: "重量 kg" }), "62.5");
  await userEvent.type(within(dialog).getByRole("textbox", { name: "感想" }), "きつい");
  await expectAccessible(dialog);
  await userEvent.click(within(dialog).getByRole("button", { name: "保存" }));
  expect(await within(dialog).findByText("保存できませんでした。")).toBeInTheDocument();
  expect(title).toHaveValue("胸の記録");
  expect(onSaved).not.toHaveBeenCalled();
  await userEvent.click(within(dialog).getByRole("button", { name: "保存" }));
  await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
  expect(body).toEqual({ title: "胸の記録", sets: [{ exercise_id: "bench", weight: 62.5, reps: 8, set_order: 1, feeling: "きつい" }] });
});
