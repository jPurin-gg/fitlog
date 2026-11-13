import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";

import CalendarPage, { CalendarPlanEditor } from "@/features/calendar/CalendarPage";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => { server.resetHandlers(); vi.useRealTimers(); });
afterAll(() => server.close());

function useCalendarHandlers(serverToday: string) {
  server.use(
    http.get(`${apiOrigin}/api/today`, () => HttpResponse.json({
      date: serverToday, timezone: "Asia/Tokyo", rest_day: false, plan: null, workout: null,
    })),
    http.get(`${apiOrigin}/api/monthly-plans`, () => HttpResponse.json([])),
    http.get(`${apiOrigin}/api/monthly-plans/:month`, () => HttpResponse.json({ status: 404 }, { status: 404 })),
    http.get(`${apiOrigin}/api/calendar`, () => HttpResponse.json({ worked_out_dates: [], worked_out_days: [], planned_days: [] })),
    http.get(`${apiOrigin}/api/workouts/by-date/:date`, () => HttpResponse.json({ status: 404 }, { status: 404 })),
    http.get(`${apiOrigin}/api/workout-plans/:date`, ({ params }) => HttpResponse.json({
      plan_date: params.date, status: "draft", plan: { workout_title: "筋トレ", target: "筋トレ", estimated_duration_min: 45, coach_note: "", exercises: [] },
    })),
  );
}

it("今日の枠と過去・未来の判定はブラウザではなくサーバーの日付を使う", async () => {
  vi.useFakeTimers({ toFake: ["Date"] });
  // どのタイムゾーンでもブラウザの日付は9/13か9/14。サーバーの今日は9/15。
  vi.setSystemTime(new Date("2026-09-13T12:00:00Z"));
  useCalendarHandlers("2026-09-15");
  const user = userEvent.setup();
  render(<CalendarPage />);

  const todayCell = await screen.findByRole("button", { name: "2026-09-15 の記録を編集" });
  await waitFor(() => expect(todayCell).toHaveAttribute("aria-current", "date"));
  expect(screen.getByRole("button", { name: "2026-09-14 の記録を編集" })).not.toHaveAttribute("aria-current");
  expect(screen.getByRole("button", { name: "2026-09-13 の記録を編集" })).not.toHaveAttribute("aria-current");

  // サーバーの今日は実施記録の編集、翌日以降は予定の編集になる。
  await user.click(todayCell);
  expect(await screen.findByRole("heading", { name: "記録を編集" })).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "キャンセル" }));
  await user.click(screen.getByRole("button", { name: "2026-09-16 の記録を編集" }));
  expect(await screen.findByRole("heading", { name: "予定を編集" })).toBeInTheDocument();
});

it("初期表示の月はサーバーの今月に合わせる", async () => {
  vi.useFakeTimers({ toFake: ["Date"] });
  // どのタイムゾーンでもブラウザはまだ8月。サーバーではすでに9/1。
  vi.setSystemTime(new Date("2026-08-30T12:00:00Z"));
  useCalendarHandlers("2026-09-01");
  render(<CalendarPage />);

  const todayCell = await screen.findByRole("button", { name: "2026-09-01 の記録を編集" });
  expect(todayCell).toHaveAttribute("aria-current", "date");
  expect(screen.getByRole("heading", { name: /2026年9月/ })).toBeInTheDocument();
});

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
