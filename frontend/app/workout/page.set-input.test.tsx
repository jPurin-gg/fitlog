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

function useStartHandlers() {
  server.use(
    http.get(`${apiOrigin}/api/exercises/:exerciseID/settings`, () => HttpResponse.json({ target_sets: 3 })),
    http.post(new RegExp(`${apiOrigin}/api/workout-plans/\\d{4}-\\d{2}-\\d{2}/start`), () => HttpResponse.json(startedPlan)),
  );
}

function useRecordHandler(bodies: unknown[]) {
  server.use(
    http.post(`${apiOrigin}/api/workouts/42/sets`, async ({ request }) => {
      const body = await request.json() as Record<string, unknown>;
      bodies.push(body);
      return HttpResponse.json({ id: 100 + bodies.length, workout_id: 42, ...body, is_pr: false, created_at: "2026-11-13T10:00:00Z" }, { status: 201 });
    }),
  );
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

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

describe("WorkoutPage set input checks", () => {
  it("小数の回数は切り捨てずにエラーにし、保存しない", async () => {
    useStartHandlers();
    const bodies: unknown[] = [];
    useRecordHandler(bodies);
    const user = userEvent.setup();
    await startWorkout(user);

    await enterSet(user, "60", "10.5");
    await user.click(screen.getByRole("button", { name: "セットを保存" }));

    expect(await screen.findByText("回数は小数ではなく整数で入力してください。")).toBeInTheDocument();
    expect(bodies).toHaveLength(0);
    expect(screen.getByPlaceholderText("10")).toBeEnabled();
  });

  it.each([
    ["2000.5", "8", "重量は0〜2,000kgの範囲で入力してください。"],
    ["60", "1001", "回数は1〜1,000回の整数で入力してください。"],
  ])("上限を超える %s kg × %s 回は保存しない", async (weight, reps, message) => {
    useStartHandlers();
    const bodies: unknown[] = [];
    useRecordHandler(bodies);
    const user = userEvent.setup();
    await startWorkout(user);

    await enterSet(user, weight, reps);
    await user.click(screen.getByRole("button", { name: "セットを保存" }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(bodies).toHaveLength(0);
  });

  it("過去最大を大きく超える重量は確認を経てから保存する", async () => {
    useStartHandlers();
    const bodies: Array<Record<string, unknown>> = [];
    useRecordHandler(bodies);
    const user = userEvent.setup();
    await startWorkout(user);

    await enterSet(user, "800", "8");
    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    const dialog = await screen.findByRole("alertdialog", { name: "800kg で保存しますか？" });
    expect(within(dialog).getByText(/過去の最大重量（80kg）/)).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "入力し直す" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(bodies).toHaveLength(0);
    expect(screen.getByPlaceholderText("80")).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "800kg で保存" }));

    expect(await screen.findByText(/セットは保存済みです/)).toBeInTheDocument();
    expect(bodies).toHaveLength(1);
    expect(bodies[0]).toMatchObject({ weight: 800, reps: 8 });
  });

  it("通常の自己ベスト更新では確認せずに保存する", async () => {
    useStartHandlers();
    const bodies: unknown[] = [];
    useRecordHandler(bodies);
    const user = userEvent.setup();
    await startWorkout(user);

    await enterSet(user, "85", "5");
    await user.click(screen.getByRole("button", { name: "セットを保存" }));

    expect(await screen.findByText(/セットは保存済みです/)).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(bodies).toHaveLength(1);
  });

  it("直前のセットは入力文字列ではなく保存APIが返した値で表示する", async () => {
    useStartHandlers();
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/sets`, () => HttpResponse.json({
        id: 101, workout_id: 42, exercise_id: "bench_press", set_order: 1, weight: 61, reps: 7, feeling: "", is_pr: false, created_at: "2026-11-13T10:00:00Z",
      }, { status: 201 })),
    );
    const user = userEvent.setup();
    await startWorkout(user);

    await enterSet(user, "62.5", "8");
    await user.click(screen.getByRole("button", { name: "セットを保存" }));

    expect(await screen.findByText("61kg × 7回")).toBeInTheDocument();
    expect(screen.queryByText("62.5kg × 8回")).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("80")).toHaveValue(61);
    expect(screen.getByPlaceholderText("10")).toHaveValue(7);
  });
});
