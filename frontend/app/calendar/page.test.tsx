import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";

import CalendarPage, { CalendarPlanEditor, CalendarWorkoutEditor } from "@/features/calendar/CalendarPage";

const apiOrigin = "http://localhost:3000";
const server = setupServer(
  http.get(`${apiOrigin}/api/monthly-plans`, () => HttpResponse.json([])),
  // 種目選択モーダルが開くと呼ばれる。検索は開いてから少し遅れて走るため、既定で空を返す。
  http.get(`${apiOrigin}/api/exercises/favorites`, () => HttpResponse.json([{ id: "bench", name: "ベンチプレス" }])),
  http.get(`${apiOrigin}/api/exercises/recent`, () => HttpResponse.json([])),
  http.get(`${apiOrigin}/api/exercises`, () => HttpResponse.json([])),
);

// 月・水・金が予定日の月間プラン。
const monthlyPlan = {
  id: 1,
  plan_month: "2026-10",
  plan_name: "週3回プラン",
  frequency: "週3回",
  description: "",
  rationale: "",
  rest_days: [],
  recommended_days: [1, 3, 5],
  weekly_routine: [
    { day_name: "Day 1", target: "胸の日", example_exercises: ["ベンチプレス"] },
    { day_name: "Day 2", target: "背中の日", example_exercises: ["懸垂"] },
    { day_name: "Day 3", target: "脚の日", example_exercises: ["スクワット"] },
  ],
};

function calendarOf(workedOutDays: { date: number; workout_id: number; type: string }[]) {
  return {
    worked_out_dates: workedOutDays.map(day => day.date),
    worked_out_days: workedOutDays,
    planned_days: [],
  };
}

function notFound(detail: string) {
  return HttpResponse.json({ status: 404, detail }, { status: 404 });
}

function deferredResponse() {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>(next => { resolve = next; });
  return { promise, resolve };
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  // 2026-10-16（金）の昼に固定する。上の月間プランでは今日が予定日になる。
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 9, 16, 12, 0, 0));
});
afterEach(() => { server.resetHandlers(); vi.useRealTimers(); });
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

it("月間プランがない未来の日付は、エラーにせず新しい予定を作成して保存できる", async () => {
  let savedBody: Record<string, unknown> | undefined;
  const onSaved = vi.fn();
  server.use(
    http.get(`${apiOrigin}/api/workout-plans/2026-12-01`, () => notFound("今月の月間プランがまだありません。")),
    http.put(`${apiOrigin}/api/workout-plans/2026-12-01`, async ({ request }) => {
      savedBody = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ id: 30, plan_date: "2026-12-01", status: "active", plan: savedBody });
    }),
  );

  const user = userEvent.setup();
  render(<CalendarPlanEditor date="2026-12-01" onClose={() => undefined} onSaved={onSaved} />);

  expect(await screen.findByRole("dialog", { name: "予定を作成" })).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(screen.queryByText("今月の月間プランがまだありません。")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "種目を追加して予定を作成" }));
  await user.click(await screen.findByRole("button", { name: "ベンチプレス" }));
  await user.click(screen.getByRole("button", { name: "予定を保存" }));

  await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
  expect(savedBody).toEqual({
    workout_title: "筋トレ",
    target: "筋トレ",
    estimated_duration_min: 45,
    coach_note: "",
    exercises: [{
      exercise_id: "bench",
      name: "ベンチプレス",
      planned_sets: 3,
      target_weight: 0,
      target_reps: 10,
    }],
  });
});

it("予定を編集した後は、背景・閉じる・キャンセルのどれでも破棄の確認を出す", async () => {
  const onClose = vi.fn();
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  server.use(http.get(`${apiOrigin}/api/workout-plans/2026-12-01`, () => HttpResponse.json({
    id: 12,
    plan_date: "2026-12-01",
    status: "active",
    plan: {
      workout_title: "胸の日",
      target: "胸",
      estimated_duration_min: 45,
      coach_note: "",
      exercises: [{ exercise_id: "bench", name: "ベンチプレス", planned_sets: 3, target_weight: 60, target_reps: 8 }],
    },
  })));

  const user = userEvent.setup();
  render(<CalendarPlanEditor date="2026-12-01" onClose={onClose} onSaved={() => undefined} />);
  const title = await screen.findByLabelText("タイトル");
  const backdrop = screen.getByRole("dialog", { name: "予定を編集" }).parentElement as HTMLElement;

  // 何も変えていなければ、確認なしで閉じる。
  await user.click(backdrop);
  expect(confirm).not.toHaveBeenCalled();
  expect(onClose).toHaveBeenCalledTimes(1);

  await user.type(title, "（変更）");
  await user.click(backdrop);
  expect(confirm).toHaveBeenCalledTimes(1);
  expect(confirm).toHaveBeenCalledWith("保存していない変更があります。破棄して閉じますか？");
  await user.click(screen.getByRole("button", { name: "閉じる" }));
  await user.click(screen.getByRole("button", { name: "キャンセル" }));
  expect(confirm).toHaveBeenCalledTimes(3);
  expect(onClose).toHaveBeenCalledTimes(1);

  confirm.mockReturnValue(true);
  await user.click(backdrop);
  expect(onClose).toHaveBeenCalledTimes(2);
});

it("記録の編集で、種目を選び直さずに同じ種目のセットを追加して保存できる", async () => {
  let savedBody: Record<string, unknown> | undefined;
  const onSaved = vi.fn();
  server.use(
    http.get(`${apiOrigin}/api/workouts/by-date/2026-10-01`, () => notFound("指定日のワークアウトがありません。")),
    http.put(`${apiOrigin}/api/workouts/by-date/2026-10-01`, async ({ request }) => {
      savedBody = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ workout_id: 9, date: "2026-10-01", title: "筋トレ", sets: [] });
    }),
  );

  const user = userEvent.setup();
  render(<CalendarWorkoutEditor date="2026-10-01" onClose={() => undefined} onSaved={onSaved} />);

  await user.click(await screen.findByRole("button", { name: "種目を追加して記録を作成" }));
  await user.click(await screen.findByRole("button", { name: "ベンチプレス" }));
  await user.clear(screen.getByLabelText("重量 kg"));
  await user.type(screen.getByLabelText("重量 kg"), "60");
  await user.click(screen.getByRole("button", { name: "同じ種目でセットを追加" }));

  expect(screen.queryByText("種目を選択")).not.toBeInTheDocument();
  expect(screen.getAllByLabelText("重量 kg")).toHaveLength(2);
  expect(screen.getAllByRole("button", { name: "同じ種目でセットを追加" })).toHaveLength(1);

  await user.click(screen.getByRole("button", { name: "保存" }));
  await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
  expect(savedBody).toEqual({
    title: "筋トレ",
    sets: [
      { exercise_id: "bench", weight: 60, reps: 10, set_order: 1, feeling: "" },
      { exercise_id: "bench", weight: 60, reps: 10, set_order: 2, feeling: "" },
    ],
  });
});

it("記録を編集した後に背景をクリックすると確認を出し、断ると閉じない", async () => {
  const onClose = vi.fn();
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  server.use(http.get(`${apiOrigin}/api/workouts/by-date/2026-10-01`, () => HttpResponse.json({
    workout_id: 7,
    date: "2026-10-01",
    title: "胸の日",
    sets: [{ id: 1, exercise_id: "bench", exercise_name: "ベンチプレス", weight: 60, reps: 8, set_order: 1, feeling: "" }],
  })));

  const user = userEvent.setup();
  render(<CalendarWorkoutEditor date="2026-10-01" onClose={onClose} onSaved={() => undefined} />);

  await user.click(await screen.findByRole("button", { name: "同じ種目でセットを追加" }));
  await user.click(screen.getByRole("dialog", { name: "記録を編集" }).parentElement as HTMLElement);

  expect(confirm).toHaveBeenCalledWith("保存していない変更があります。破棄して閉じますか？");
  expect(onClose).not.toHaveBeenCalled();
});

it("予定は今日以降だけに表示し、過去の月の未実施日を予定として表示しない", async () => {
  server.use(
    http.get(`${apiOrigin}/api/monthly-plans/:month`, () => HttpResponse.json(monthlyPlan)),
    http.get(`${apiOrigin}/api/calendar`, ({ request }) => {
      const month = new URL(request.url).searchParams.get("month");
      if (month === "10") return HttpResponse.json(calendarOf([{ date: 12, workout_id: 1, type: "胸の日" }]));
      if (month === "9") return HttpResponse.json(calendarOf([{ date: 7, workout_id: 2, type: "胸の日" }]));
      return HttpResponse.json(calendarOf([]));
    }),
  );

  const user = userEvent.setup();
  render(<CalendarPage />);

  // 当月: 過ぎた予定日は未実施、今日とその先は予定。
  expect(await screen.findByRole("button", { name: "2026-10-12 の記録を編集（実施済み）" })).toBeInTheDocument();
  expect(await screen.findByRole("button", { name: "2026-10-14 の記録を編集（未実施）" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "2026-10-16 の記録を編集（予定）" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "2026-10-19 の記録を編集（予定）" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "2026-10-15 の記録を編集" })).toBeInTheDocument();

  // 過去の月: 予定日だった未実施日は「未実施」で、予定の表示にはしない。
  await user.click(screen.getByRole("button", { name: "前の月" }));
  const missed = await screen.findByRole("button", { name: "2026-09-09 の記録を編集（未実施）" });
  expect(within(missed).getByText("未実施")).toBeInTheDocument();
  expect(within(missed).queryByText("背中の日")).not.toBeInTheDocument();
  expect(await screen.findByRole("button", { name: "2026-09-07 の記録を編集（実施済み）" })).toBeInTheDocument();
  expect(screen.queryAllByRole("button", { name: /^2026-09-\d{2} の記録を編集（予定）$/ })).toHaveLength(0);

  // 未来の月: 予定日はすべて予定。
  await user.click(screen.getByRole("button", { name: "次の月" }));
  await user.click(screen.getByRole("button", { name: "次の月" }));
  expect(await screen.findByRole("button", { name: "2026-11-02 の記録を編集（予定）" })).toBeInTheDocument();
  expect(screen.queryAllByRole("button", { name: /^2026-11-\d{2} の記録を編集（未実施）$/ })).toHaveLength(0);
});

it("月を切り替えた直後は前の月の実施マークとプランを表示せず、読み込み中を表示する", async () => {
  const september = deferredResponse();
  server.use(
    http.get(`${apiOrigin}/api/monthly-plans/:month`, ({ params }) => params.month === "2026-10"
      ? HttpResponse.json(monthlyPlan)
      : notFound("月間プランがまだありません。")),
    http.get(`${apiOrigin}/api/calendar`, async ({ request }) => {
      const month = new URL(request.url).searchParams.get("month");
      if (month === "9") return september.promise;
      return HttpResponse.json(calendarOf([{ date: 12, workout_id: 1, type: "10月の記録" }]));
    }),
  );

  const user = userEvent.setup();
  render(<CalendarPage />);
  await screen.findByRole("button", { name: "2026-10-12 の記録を編集（実施済み）" });
  await screen.findByText("週3回プラン");

  await user.click(screen.getByRole("button", { name: "前の月" }));

  // 9月の応答を待たずに、10月のマークとプランが消えている。
  expect(screen.getByRole("button", { name: "2026-09-12 の記録を編集" })).toBeInTheDocument();
  expect(screen.queryByText("10月の記録")).not.toBeInTheDocument();
  expect(screen.queryByText("週3回プラン")).not.toBeInTheDocument();
  expect(screen.getByText("実施記録と予定を読み込み中...")).toBeInTheDocument();

  september.resolve(HttpResponse.json(calendarOf([{ date: 3, workout_id: 2, type: "9月の記録" }])));
  expect(await screen.findByRole("button", { name: "2026-09-03 の記録を編集（実施済み）" })).toBeInTheDocument();
  expect(screen.queryByText("実施記録と予定を読み込み中...")).not.toBeInTheDocument();
});

it("古い月の応答が後から届いても、表示中の月を上書きしない", async () => {
  const september = deferredResponse();
  let septemberStarted = false;
  let septemberFinished = false;
  server.use(
    http.get(`${apiOrigin}/api/monthly-plans/:month`, () => notFound("月間プランがまだありません。")),
    http.get(`${apiOrigin}/api/calendar`, async ({ request }) => {
      const month = new URL(request.url).searchParams.get("month");
      if (month === "9") {
        septemberStarted = true;
        const response = await september.promise;
        septemberFinished = true;
        return response;
      }
      if (month === "8") return HttpResponse.json(calendarOf([{ date: 20, workout_id: 3, type: "8月の記録" }]));
      return HttpResponse.json(calendarOf([]));
    }),
  );

  const user = userEvent.setup();
  render(<CalendarPage />);

  await user.click(screen.getByRole("button", { name: "前の月" }));
  await waitFor(() => expect(septemberStarted).toBe(true));
  await user.click(screen.getByRole("button", { name: "前の月" }));
  await screen.findByRole("button", { name: "2026-08-20 の記録を編集（実施済み）" });

  september.resolve(HttpResponse.json(calendarOf([{ date: 3, workout_id: 2, type: "9月の記録" }])));
  await waitFor(() => expect(septemberFinished).toBe(true));
  // 遅れた応答が画面へ届きうる時間を置いてから確認する。
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 50)); });

  expect(screen.getByRole("button", { name: "2026-08-20 の記録を編集（実施済み）" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "2026-08-03 の記録を編集" })).toBeInTheDocument();
  expect(screen.queryByText("9月の記録")).not.toBeInTheDocument();
});

it("取得に失敗した月では前の月の表示を残さず、失敗を表示して再読み込みできる", async () => {
  const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
  let septemberFails = true;
  server.use(
    http.get(`${apiOrigin}/api/monthly-plans/:month`, ({ params }) => {
      if (params.month === "2026-10") return HttpResponse.json(monthlyPlan);
      if (septemberFails) return HttpResponse.json({ status: 500, detail: "unavailable" }, { status: 500 });
      return HttpResponse.json({ ...monthlyPlan, plan_month: "2026-09", plan_name: "9月のプラン" });
    }),
    http.get(`${apiOrigin}/api/calendar`, ({ request }) => {
      const month = new URL(request.url).searchParams.get("month");
      if (month === "10") return HttpResponse.json(calendarOf([{ date: 12, workout_id: 1, type: "10月の記録" }]));
      if (septemberFails) return HttpResponse.json({ status: 500, detail: "unavailable" }, { status: 500 });
      return HttpResponse.json(calendarOf([{ date: 3, workout_id: 2, type: "9月の記録" }]));
    }),
  );

  const user = userEvent.setup();
  render(<CalendarPage />);
  await screen.findByRole("button", { name: "2026-10-12 の記録を編集（実施済み）" });
  await screen.findByText("週3回プラン");

  await user.click(screen.getByRole("button", { name: "前の月" }));

  expect(await screen.findByText("実施記録と予定を読み込めませんでした。")).toBeInTheDocument();
  expect(await screen.findByText("2026-09 の月間プランを読み込めませんでした。")).toBeInTheDocument();
  expect(screen.queryByText("10月の記録")).not.toBeInTheDocument();
  expect(screen.queryByText("週3回プラン")).not.toBeInTheDocument();
  expect(screen.queryAllByRole("button", { name: /^2026-09-\d{2} の記録を編集（.+）$/ })).toHaveLength(0);
  expect(consoleError).toHaveBeenCalled();

  septemberFails = false;
  await user.click(screen.getByRole("button", { name: "カレンダーを再読み込み" }));
  expect(await screen.findByRole("button", { name: "2026-09-03 の記録を編集（実施済み）" })).toBeInTheDocument();
  expect(screen.queryByText("実施記録と予定を読み込めませんでした。")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "プランを再読み込み" }));
  expect(await screen.findByText("9月のプラン")).toBeInTheDocument();
});

it("月間プランがなくても、未来の日付に予定を作成するとカレンダーへ反映する", async () => {
  let saved = false;
  server.use(
    http.get(`${apiOrigin}/api/monthly-plans/:month`, () => notFound("月間プランがまだありません。")),
    http.get(`${apiOrigin}/api/calendar`, () => HttpResponse.json(saved
      ? { worked_out_dates: [], worked_out_days: [], planned_days: [{ date: 20, plan_id: 30, target: "筋トレ" }] }
      : calendarOf([]))),
    http.get(`${apiOrigin}/api/workout-plans/2026-10-20`, () => notFound("今月の月間プランがまだありません。")),
    http.put(`${apiOrigin}/api/workout-plans/2026-10-20`, async ({ request }) => {
      const body = await request.json() as Record<string, unknown>;
      saved = true;
      return HttpResponse.json({ id: 30, plan_date: "2026-10-20", status: "active", plan: body });
    }),
  );

  const user = userEvent.setup();
  render(<CalendarPage />);

  await user.click(screen.getByRole("button", { name: "2026-10-20 の記録を編集" }));
  await screen.findByRole("dialog", { name: "予定を作成" });
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "種目を追加して予定を作成" }));
  await user.click(await screen.findByRole("button", { name: "ベンチプレス" }));
  await user.click(screen.getByRole("button", { name: "予定を保存" }));

  expect(await screen.findByRole("button", { name: "2026-10-20 の記録を編集（予定）" })).toBeInTheDocument();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("「今日の記録」はボタンで、キーボードから今日の記録を開ける", async () => {
  server.use(
    http.get(`${apiOrigin}/api/monthly-plans/:month`, () => notFound("月間プランがまだありません。")),
    http.get(`${apiOrigin}/api/calendar`, () => HttpResponse.json(calendarOf([]))),
    http.get(`${apiOrigin}/api/workouts/by-date/2026-10-16`, () => notFound("指定日のワークアウトがありません。")),
  );

  const user = userEvent.setup();
  render(<CalendarPage />);

  const todayButton = screen.getByRole("button", { name: /今日の記録/ });
  todayButton.focus();
  await user.keyboard("{Enter}");

  const dialog = await screen.findByRole("dialog", { name: "記録を編集" });
  expect(within(dialog).getByText("2026-10-16")).toBeInTheDocument();
});
