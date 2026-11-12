import React from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import CalendarPage from "./CalendarPage";

const origin = "http://localhost:3000";
const calendarData = { worked_out_dates: [5], worked_out_days: [{ date: 5, workout_id: 12, type: "上半身の記録" }] };
const plan = { plan_name: "保存済みプラン", frequency: "週1回", recommended_days: [4], weekly_routine: [] };
const server = setupServer(
  http.get(`${origin}/api/calendar`, () => HttpResponse.json(calendarData)),
  http.get(`${origin}/api/monthly-plans`, () => HttpResponse.json([])),
  http.get(`${origin}/api/monthly-plans/:month`, () => HttpResponse.json({}, { status: 404 })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-01T12:00:00Z"));
});
afterEach(() => { server.resetHandlers(); vi.useRealTimers(); });
afterAll(() => server.close());

it.each([
  { path: "/api/calendar", message: "カレンダーの取得エラー", loading: "カレンダーを読み込み中...", result: calendarData, visible: "上半身の記録" },
  { path: "/api/monthly-plans", message: "履歴の取得エラー", loading: "プラン履歴を読み込み中...", result: [{ ...plan, plan_month: "2026-09" }], visible: "保存済みプラン" },
  { path: "/api/monthly-plans/2026-10", message: "月間プランの取得エラー", loading: "月間プランを読み込み中...", result: plan, visible: "保存済みプラン" },
])("$path の失敗を空データと区別し、再試行できる", async ({ path, message, loading, result, visible }) => {
  let calls = 0;
  server.use(http.get(`${origin}${path}`, () => {
    calls++;
    return calls === 1 ? HttpResponse.json({ detail: message }, { status: 500 }) : HttpResponse.json(result);
  }));
  render(<CalendarPage />);
  const error = await screen.findByRole("alert");
  expect(error).toHaveTextContent(message);
  expect(screen.queryByText(loading)).not.toBeInTheDocument();
  await userEvent.click(within(error).getByRole("button", { name: "再試行" }));
  expect(await screen.findByText(visible)).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(calls).toBe(2);
});

it("月間プランの404は未作成として表示する", async () => {
  render(<CalendarPage />);
  expect(await screen.findByText(/2026-10 のプランはまだありません/)).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("月を切り替えた後に前月の遅い応答を表示しない", async () => {
  let resolveCalendar!: (response: Response) => void;
  let resolvePlan!: (response: Response) => void;
  const previousCalendar = new Promise<Response>(resolve => { resolveCalendar = resolve; });
  const previousPlan = new Promise<Response>(resolve => { resolvePlan = resolve; });
  let previousCalls = 0;
  server.use(
    http.get(`${origin}/api/calendar`, ({ request }) => {
      if (new URL(request.url).searchParams.get("month") === "10") {
        previousCalls++;
        return previousCalendar;
      }
      return HttpResponse.json({ worked_out_dates: [], worked_out_days: [] });
    }),
    http.get(`${origin}/api/monthly-plans/:month`, ({ params }) => {
      if (params.month === "2026-10") {
        previousCalls++;
        return previousPlan;
      }
      return HttpResponse.json({}, { status: 404 });
    }),
  );
  render(<CalendarPage />);
  await waitFor(() => expect(previousCalls).toBe(2));
  await userEvent.click(screen.getByRole("button", { name: "次の月" }));
  await screen.findByText(/2026-11 のプランはまだありません/);
  await act(async () => {
    resolveCalendar(HttpResponse.json(calendarData));
    resolvePlan(HttpResponse.json(plan));
  });
  expect(screen.queryByText("上半身の記録")).not.toBeInTheDocument();
  expect(screen.queryByText("保存済みプラン")).not.toBeInTheDocument();
  expect(screen.getByText(/2026-11 のプランはまだありません/)).toBeInTheDocument();
});
