import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";

import { CalendarPlanEditor } from "@/features/calendar/CalendarPage";

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

  await user.clear(await screen.findByLabelText("タイトル"));
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
