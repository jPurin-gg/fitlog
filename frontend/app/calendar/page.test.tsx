import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";

import { CalendarPlanEditor } from "@/features/calendar/CalendarPage";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function usePlanHandler(plannedSets = 3) {
  server.use(http.get(`${apiOrigin}/api/workout-plans/2026-09-20`, () => HttpResponse.json({
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
        planned_sets: plannedSets,
        target_weight: 60,
        target_reps: 8,
      }],
    },
  })));
}

it.each([1, 20])("未来の予定を編集し、%iセットを日次プランとして保存する", async plannedSets => {
  let savedBody: Record<string, unknown> | undefined;
  const onSaved = vi.fn();
  usePlanHandler();
  server.use(
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
  const sets = screen.getByLabelText("セット");
  expect(sets).toHaveAttribute("min", "1");
  expect(sets).toHaveAttribute("max", "20");
  await user.clear(sets);
  await user.type(sets, String(plannedSets));
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
      planned_sets: plannedSets,
      target_weight: 62.5,
      target_reps: 8,
    }],
  });
});

it.each([0, -1, 21, 4294967296, 1.5])("範囲外のセット数 %s は送信しない", async plannedSets => {
  usePlanHandler();
  const put = vi.fn(() => HttpResponse.json({}));
  const onSaved = vi.fn();
  server.use(http.put(`${apiOrigin}/api/workout-plans/2026-09-20`, put));
  const user = userEvent.setup();
  render(<CalendarPlanEditor date="2026-09-20" onClose={() => undefined} onSaved={onSaved} />);

  const sets = await screen.findByLabelText("セット");
  fireEvent.change(sets, { target: { value: String(plannedSets) } });
  await user.click(screen.getByRole("button", { name: "予定を保存" }));

  expect(await screen.findByText("予定のセット数は1〜20の整数で指定してください。")).toBeInTheDocument();
  expect(put).not.toHaveBeenCalled();
  expect(onSaved).not.toHaveBeenCalled();
});

it("既存予定の巨大なセット数も表示でき、範囲内へ修正して保存できる", async () => {
  usePlanHandler(4294967296);
  const onSaved = vi.fn();
  server.use(http.put(`${apiOrigin}/api/workout-plans/2026-09-20`, () => HttpResponse.json({})));
  const user = userEvent.setup();
  render(<CalendarPlanEditor date="2026-09-20" onClose={() => undefined} onSaved={onSaved} />);

  const sets = await screen.findByLabelText("セット");
  expect(sets).toHaveValue(4294967296);
  await user.clear(sets);
  await user.type(sets, "20");
  await user.click(screen.getByRole("button", { name: "予定を保存" }));
  expect(onSaved).toHaveBeenCalledOnce();
});
