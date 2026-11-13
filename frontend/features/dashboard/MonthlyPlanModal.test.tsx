import React from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AISettingsProvider } from "@/components/AISettings";
import { MonthlyPlanModal } from "./DashboardPage";
import { DAILY_PLAN_NOTE } from "./monthlyPlanForm";

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function show(onPlanGenerated = vi.fn()) {
  render(<AISettingsProvider userID={1}><MonthlyPlanModal onClose={vi.fn()} onPlanGenerated={onPlanGenerated} /></AISettingsProvider>);
}

it("案内を開くだけでは生成せず、ONにして明示的に作成できる", async () => {
  const onPlanGenerated = vi.fn();
  let calls = 0;
  const plan = { plan_name: "今月のプラン" };
  server.use(http.post("http://localhost:3000/api/monthly-plans/:month/generate", () => {
    calls++;
    return HttpResponse.json(plan);
  }));
  show(onPlanGenerated);
  expect(screen.getByRole("button", { name: "決定してプランを作成" })).toBeDisabled();
  expect(calls).toBe(0);
  await userEvent.click(screen.getByRole("switch"));
  expect(calls).toBe(0);
  await userEvent.click(screen.getByRole("button", { name: "決定してプランを作成" }));
  await waitFor(() => expect(onPlanGenerated).toHaveBeenCalledWith(plan));
  expect(calls).toBe(1);
});

it("生成中にOFF→ONにしても古いプランを反映せず、再生成しない", async () => {
  let resolve!: (response: Response) => void;
  const pending = new Promise<Response>(next => { resolve = next; });
  const onPlanGenerated = vi.fn();
  let calls = 0;
  let finished = false;
  server.use(http.post("http://localhost:3000/api/monthly-plans/:month/generate", async () => {
    calls++;
    const response = await pending;
    finished = true;
    return response;
  }));
  show(onPlanGenerated);
  const user = userEvent.setup();
  await user.click(screen.getByRole("switch"));
  await user.click(screen.getByRole("button", { name: "決定してプランを作成" }));
  await waitFor(() => expect(calls).toBe(1));
  await user.click(screen.getByRole("switch"));
  await user.click(screen.getByRole("switch"));
  await act(async () => { resolve(HttpResponse.json({ plan_name: "古いプラン" })); await pending; });
  await waitFor(() => expect(finished).toBe(true));
  expect(onPlanGenerated).not.toHaveBeenCalled();
  expect(calls).toBe(1);
  expect(screen.getByRole("button", { name: "決定してプランを作成" })).toBeEnabled();
});

const currentPlan = {
  id: 3,
  plan_month: "2026-10",
  plan_name: "手で調整した全身プラン",
  frequency: "週5-6回",
  description: "全身をバランスよく鍛えます。",
  rationale: "継続を優先します。 土曜・日曜は休息日として避けて、実施曜日を調整しました。",
  rest_days: [0, 6],
  recommended_days: [1, 3],
  weekly_routine: [
    { day_name: "1日目", target: "押す日", example_exercises: ["ベンチプレス", "ショルダープレス"], exercise_ids: ["bench", "press"] },
    { day_name: "2日目", target: "脚の日", example_exercises: ["スクワット"], exercise_ids: ["squat"] },
  ],
};

function showCurrent(onPlanGenerated = vi.fn()) {
  render(
    <AISettingsProvider userID={1}>
      <MonthlyPlanModal month="2026-10" currentPlan={currentPlan} onClose={vi.fn()} onPlanGenerated={onPlanGenerated} />
    </AISettingsProvider>,
  );
}

it("現行プランの名前・頻度・休息日・週の構成・説明・根拠と日次予定の扱いを確認できる", () => {
  showCurrent();
  const dialog = screen.getByRole("dialog", { name: "月間プラン" });
  expect(within(dialog).getByRole("heading", { name: "手で調整した全身プラン" })).toBeInTheDocument();
  expect(within(dialog).getByText("週5-6回")).toBeInTheDocument();
  expect(within(dialog).getByText("日曜・土曜")).toBeInTheDocument();
  const week = within(dialog).getByRole("region", { name: "週の構成" });
  const days = within(week).getAllByRole("listitem");
  expect(days).toHaveLength(2);
  expect(days[0]).toHaveTextContent("月曜");
  expect(days[0]).toHaveTextContent("押す日");
  expect(days[0]).toHaveTextContent("ベンチプレス・ショルダープレス");
  expect(days[1]).toHaveTextContent("水曜");
  expect(days[1]).toHaveTextContent("スクワット");
  expect(within(dialog).getByRole("region", { name: "説明" })).toHaveTextContent(currentPlan.description);
  expect(within(dialog).getByRole("region", { name: "根拠" })).toHaveTextContent("土曜・日曜は休息日として避けて、実施曜日を調整しました。");
  expect(dialog).toHaveTextContent(DAILY_PLAN_NOTE);
  // 確認画面からは直接生成できない。
  expect(within(dialog).queryByRole("button", { name: /作成|この条件で作り直す/ })).not.toBeInTheDocument();
});

it("作り直しの初期値は現行プランの頻度と休息日になる", async () => {
  showCurrent();
  await userEvent.click(screen.getByRole("button", { name: "作り直す" }));
  expect(screen.getByRole("heading", { name: "月間プランを作り直す" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "週5〜6回" })).toHaveAttribute("aria-pressed", "true");
  expect(screen.getByRole("button", { name: "週3〜4回" })).toHaveAttribute("aria-pressed", "false");
  for (const label of ["日", "土"]) expect(screen.getByRole("button", { name: label })).toHaveAttribute("aria-pressed", "true");
  for (const label of ["月", "火", "水", "木", "金"]) expect(screen.getByRole("button", { name: label })).toHaveAttribute("aria-pressed", "false");
  await userEvent.click(screen.getByRole("button", { name: "現在のプランに戻る" }));
  expect(screen.getByRole("heading", { name: "手で調整した全身プラン" })).toBeInTheDocument();
});

it("当月プランがあると上書きを確認し、キャンセルでは生成しない", async () => {
  const onPlanGenerated = vi.fn();
  const requests: unknown[] = [];
  const generated = { ...currentPlan, plan_name: "新しいプラン" };
  server.use(http.post("http://localhost:3000/api/monthly-plans/:month/generate", async ({ request }) => {
    requests.push(await request.json());
    return HttpResponse.json(generated);
  }));
  showCurrent(onPlanGenerated);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "作り直す" }));
  await user.click(screen.getByRole("switch"));
  await user.click(screen.getByRole("button", { name: "この条件で作り直す" }));

  const confirm = screen.getByRole("alertdialog", { name: "上書きの確認" });
  expect(confirm).toHaveTextContent("「手で調整した全身プラン」は新しいプランで置き換わり、元に戻せません。");
  expect(confirm).toHaveTextContent(DAILY_PLAN_NOTE);
  expect(within(confirm).getByRole("button", { name: "キャンセル" })).toHaveFocus();
  await user.click(within(confirm).getByRole("button", { name: "キャンセル" }));
  expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  expect(requests).toEqual([]);
  expect(onPlanGenerated).not.toHaveBeenCalled();

  await user.click(screen.getByRole("button", { name: "この条件で作り直す" }));
  await user.click(screen.getByRole("button", { name: "上書きして作り直す" }));
  await waitFor(() => expect(onPlanGenerated).toHaveBeenCalledWith(generated));
  expect(requests).toEqual([{ motivation: "健康維持", frequency: "週5-6回", rest_days: [0, 6] }]);
});

it("当月プランの有無が未確定なら、プランがなくても上書きを確認する", async () => {
  let calls = 0;
  server.use(http.post("http://localhost:3000/api/monthly-plans/:month/generate", () => {
    calls++;
    return HttpResponse.json({ plan_name: "今月のプラン" });
  }));
  render(<AISettingsProvider userID={1}><MonthlyPlanModal confirmOverwrite onClose={vi.fn()} onPlanGenerated={vi.fn()} /></AISettingsProvider>);
  await userEvent.click(screen.getByRole("switch"));
  await userEvent.click(screen.getByRole("button", { name: "決定してプランを作成" }));
  expect(screen.getByRole("alertdialog", { name: "上書きの確認" })).toHaveTextContent("今月のプランがすでにある場合は");
  await userEvent.click(screen.getByRole("button", { name: "キャンセル" }));
  expect(calls).toBe(0);
});
