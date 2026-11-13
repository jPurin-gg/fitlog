import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import WorkoutPage from "./page";
import { AISettingsProvider } from "@/components/AISettings";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

const startedPlan = {
  id: 10,
  workout_id: 42,
  plan_date: "2026-11-13",
  status: "active",
  ai_status: "not_requested",
  plan: {
    workout_title: "今日のプラン",
    target: "胸",
    estimated_duration_min: 30,
    coach_note: "記録を優先しましょう。",
    exercises: [{
      exercise_id: "bench_press",
      name: "ベンチプレス",
      planned_sets: 3,
      target_weight: 60,
      target_reps: 8,
      last_max_weight: 80,
    }],
  },
};

interface Calls {
  recorded: Array<Record<string, unknown>>;
  patched: Array<{ setID: string; body: Record<string, unknown> }>;
  deleted: string[];
}

function useHandlers(calls: Calls) {
  server.use(
    http.get(`${apiOrigin}/api/exercises/:exerciseID/settings`, () => HttpResponse.json({ target_sets: 3 })),
    http.post(new RegExp(`${apiOrigin}/api/workout-plans/\\d{4}-\\d{2}-\\d{2}/start`), () => HttpResponse.json(startedPlan)),
    http.post(`${apiOrigin}/api/workouts/42/sets`, async ({ request }) => {
      const body = await request.json() as Record<string, unknown>;
      calls.recorded.push(body);
      return HttpResponse.json({ id: 100 + calls.recorded.length, workout_id: 42, ...body, is_pr: false, created_at: "2026-11-13T10:00:00Z" }, { status: 201 });
    }),
    http.patch(`${apiOrigin}/api/workouts/42/sets/:setID`, async ({ request, params }) => {
      const body = await request.json() as Record<string, unknown>;
      calls.patched.push({ setID: String(params.setID), body });
      return HttpResponse.json({
        id: Number(params.setID), workout_id: 42, exercise_id: "bench_press", set_order: 1,
        weight: body.weight, reps: body.reps, feeling: body.feeling ?? "", is_pr: false, created_at: "2026-11-13T10:00:00Z",
      });
    }),
    http.delete(`${apiOrigin}/api/workouts/42/sets/:setID`, ({ params }) => {
      calls.deleted.push(String(params.setID));
      return new HttpResponse(null, { status: 204 });
    }),
  );
}

function newCalls(): Calls {
  return { recorded: [], patched: [], deleted: [] };
}

async function startWorkout(user: ReturnType<typeof userEvent.setup>) {
  window.localStorage.setItem("fitlog:ai-enabled:1", "false");
  render(<AISettingsProvider userID={1}><WorkoutPage /></AISettingsProvider>);
  await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
  await screen.findByRole("heading", { name: "ワークアウト中" });
}

async function enterSet(user: ReturnType<typeof userEvent.setup>, weight: string, reps: string) {
  await user.clear(screen.getByPlaceholderText("80"));
  await user.type(screen.getByPlaceholderText("80"), weight);
  await user.clear(screen.getByPlaceholderText("10"));
  await user.type(screen.getByPlaceholderText("10"), reps);
}

async function saveSet(user: ReturnType<typeof userEvent.setup>, weight: string, reps: string) {
  await enterSet(user, weight, reps);
  await user.click(screen.getByRole("button", { name: "セットを保存" }));
  await screen.findByText(/セットは保存済みです/);
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

describe("WorkoutPage saved set correction", () => {
  it("保存済みの直前のセットを記録中に修正し、保存値で表示し直す", async () => {
    const calls = newCalls();
    useHandlers(calls);
    const user = userEvent.setup();
    await startWorkout(user);
    await saveSet(user, "70", "8");

    expect(screen.getByPlaceholderText("80")).toBeDisabled();
    await user.click(screen.getByRole("button", { name: /このセットを修正/ }));
    expect(screen.getByText("保存済みの1セット目を修正中です。直した値で保存すると、自己ベストと最大重量も再計算されます。")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /同じ内容で次へ/ })).not.toBeInTheDocument();

    await enterSet(user, "72.5", "6");
    await user.click(screen.getByRole("button", { name: "修正を保存" }));

    expect(await screen.findByText(/セットを修正しました/)).toBeInTheDocument();
    expect(calls.patched).toEqual([{ setID: "101", body: { weight: 72.5, reps: 6, feeling: "" } }]);
    expect(calls.recorded).toHaveLength(1);
    expect(screen.getByText("72.5kg × 6回")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("80")).toBeDisabled();
    expect(screen.getByRole("button", { name: /同じ内容で次へ/ })).toBeEnabled();
  });

  it("修正でも小数の回数と上限超過は送信前にエラーにする", async () => {
    const calls = newCalls();
    useHandlers(calls);
    const user = userEvent.setup();
    await startWorkout(user);
    await saveSet(user, "70", "8");

    await user.click(screen.getByRole("button", { name: /このセットを修正/ }));
    await enterSet(user, "70", "8.5");
    await user.click(screen.getByRole("button", { name: "修正を保存" }));
    expect(await screen.findByText("回数は小数ではなく整数で入力してください。")).toBeInTheDocument();

    await enterSet(user, "2001", "8");
    await user.click(screen.getByRole("button", { name: "修正を保存" }));
    expect(await screen.findByText("重量は0〜2,000kgの範囲で入力してください。")).toBeInTheDocument();
    expect(calls.patched).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "修正をやめる" }));
    expect(screen.getByPlaceholderText("80")).toHaveValue(70);
    expect(screen.getByPlaceholderText("10")).toHaveValue(8);
    expect(screen.getByPlaceholderText("80")).toBeDisabled();
  });

  it("修正後の重量が過去最大を大きく超える場合は確認を経てから送信する", async () => {
    const calls = newCalls();
    useHandlers(calls);
    const user = userEvent.setup();
    await startWorkout(user);
    await saveSet(user, "70", "8");

    await user.click(screen.getByRole("button", { name: /このセットを修正/ }));
    await enterSet(user, "700", "8");
    await user.click(screen.getByRole("button", { name: "修正を保存" }));
    const dialog = await screen.findByRole("alertdialog", { name: "700kg で保存しますか？" });
    expect(within(dialog).getByText(/過去の最大重量（80kg）/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "入力し直す" }));
    expect(calls.patched).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "修正を保存" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "700kg で保存" }));
    expect(await screen.findByText(/セットを修正しました/)).toBeInTheDocument();
    expect(calls.patched).toHaveLength(1);
    expect(calls.patched[0].body).toMatchObject({ weight: 700, reps: 8 });
  });

  it("保存済みのセットを確認後に取り消し、同じセット番号で記録し直せる", async () => {
    const calls = newCalls();
    useHandlers(calls);
    const user = userEvent.setup();
    await startWorkout(user);
    await saveSet(user, "70", "8");

    await user.click(screen.getByRole("button", { name: /このセットを取り消す/ }));
    const keep = await screen.findByRole("alertdialog", { name: "このセットを取り消しますか？" });
    await user.click(within(keep).getByRole("button", { name: "残す" }));
    expect(calls.deleted).toHaveLength(0);
    expect(screen.getByText(/セットは保存済みです/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /このセットを取り消す/ }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "取り消す" }));

    expect(await screen.findByText("1セット目の記録を取り消しました。内容を直して保存し直せます。")).toBeInTheDocument();
    expect(calls.deleted).toEqual(["101"]);
    expect(screen.getByText(/セット 1 \//)).toBeInTheDocument();
    expect(screen.getByPlaceholderText("80")).toBeEnabled();

    await saveSet(user, "72.5", "8");
    expect(calls.recorded).toHaveLength(2);
    expect(calls.recorded[1]).toMatchObject({ set_order: 1, weight: 72.5, reps: 8 });
  });

  it("修正・取り消しの失敗はエラーを表示し、保存済みの値を変えない", async () => {
    const calls = newCalls();
    useHandlers(calls);
    server.use(
      http.patch(`${apiOrigin}/api/workouts/42/sets/:setID`, () => HttpResponse.json(
        { title: "Conflict", status: 409, detail: "終了したワークアウトのセットはカレンダーから編集してください。" },
        { status: 409, headers: { "Content-Type": "application/problem+json" } },
      )),
    );
    const user = userEvent.setup();
    await startWorkout(user);
    await saveSet(user, "70", "8");

    await user.click(screen.getByRole("button", { name: /このセットを修正/ }));
    await enterSet(user, "65", "8");
    await user.click(screen.getByRole("button", { name: "修正を保存" }));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "修正を保存" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "修正をやめる" }));
    expect(screen.getByText("70kg × 8回")).toBeInTheDocument();
  });
});
