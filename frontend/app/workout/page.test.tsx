import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import WorkoutPage from "./page";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

const startedPlan = {
  id: 10,
  workout_id: 42,
  plan_date: "2026-08-24",
  status: "active",
  ai_status: "applied",
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
      last_max_weight: 75,
    }],
  },
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(next => {
    resolve = next;
  });
  return { promise, resolve };
}

function useCommonHandlers(aiStatus: "applied" | "fallback" | "not_requested" = "applied") {
  server.use(
    http.get(`${apiOrigin}/api/exercises/:exerciseID/settings`, () => HttpResponse.json({ target_sets: 3 })),
    http.post(new RegExp(`${apiOrigin}/api/workout-plans/\\d{4}-\\d{2}-\\d{2}/start`), () => (
      HttpResponse.json({ ...startedPlan, ai_status: aiStatus })
    )),
  );
}

async function startWorkout(user: ReturnType<typeof userEvent.setup>) {
  render(<WorkoutPage />);
  await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
  await screen.findByRole("heading", { name: "ワークアウト中" });
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

describe("WorkoutPage record-first flow", () => {
  it("セット保存を先に確定し、AIを待たずに次セットへ進める", async () => {
    useCommonHandlers();
    const recommendation = deferred<Response>();
    let delayedHandlerFinished = false;
    let idempotencyKey = "";
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/sets`, async ({ request }) => {
        idempotencyKey = request.headers.get("Idempotency-Key") || "";
        return HttpResponse.json({ id: 100, workout_id: 42 });
      }),
      http.post(`${apiOrigin}/api/workouts/42/sets/100/recommendation`, async () => {
        const response = await recommendation.promise;
        delayedHandlerFinished = true;
        return response;
      }),
    );
    const user = userEvent.setup();
    await startWorkout(user);

    await user.click(screen.getByRole("button", { name: "セットを保存" }));

    expect(await screen.findByText(/セットは保存済みです/)).toBeInTheDocument();
    expect(screen.getByText("AI提案を取得中")).toBeInTheDocument();
    expect(idempotencyKey.length).toBeGreaterThanOrEqual(8);

    await user.click(screen.getByRole("button", { name: "AIを待たず次セットへ" }));
    expect(await screen.findByText("セット 2 / 3")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("80")).toHaveValue(60);
    expect(screen.getByPlaceholderText("10")).toHaveValue(8);

    await act(async () => {
      recommendation.resolve(HttpResponse.json({
        next_action: "CONTINUE",
        recommendation: "遅い応答",
        target_weight: 62.5,
        target_reps: 8,
        reason: "test",
        max_weight: 75,
      }));
      await recommendation.promise;
    });
    await waitFor(() => expect(delayedHandlerFinished).toBe(true));
    expect(screen.queryByText("遅い応答")).not.toBeInTheDocument();
  });

  it("AI提案だけを再試行し、保存済みセットは再送しない", async () => {
    useCommonHandlers();
    let saveCalls = 0;
    let recommendationCalls = 0;
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/sets`, () => {
        saveCalls += 1;
        return HttpResponse.json({ id: 101, workout_id: 42 });
      }),
      http.post(`${apiOrigin}/api/workouts/42/sets/101/recommendation`, () => {
        recommendationCalls += 1;
        if (recommendationCalls === 1) {
          return HttpResponse.json(
            { status: 503, detail: "AIが一時的に使えません。" },
            { status: 503, headers: { "Content-Type": "application/problem+json" } },
          );
        }
        return HttpResponse.json({
          next_action: "CONTINUE",
          recommendation: "同じ重量で続けましょう。",
          target_weight: 60,
          target_reps: 8,
          reason: "安定しているため",
          max_weight: 75,
        });
      }),
    );
    const user = userEvent.setup();
    await startWorkout(user);

    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    expect(await screen.findByText("AI提案のみ取得できませんでした")).toBeInTheDocument();
    expect(screen.getByText(/セットは保存済みです/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "AI提案だけ再試行" }));
    expect(await screen.findByText("同じ重量で続けましょう。")).toBeInTheDocument();
    expect(saveCalls).toBe(1);
    expect(recommendationCalls).toBe(2);
  });

  it("保存の通信失敗後も入力と冪等キーを保って再送する", async () => {
    useCommonHandlers();
    const keys: string[] = [];
    let saveCalls = 0;
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/sets`, ({ request }) => {
        keys.push(request.headers.get("Idempotency-Key") || "");
        saveCalls += 1;
        if (saveCalls === 1) return HttpResponse.error();
        return HttpResponse.json({ id: 102, workout_id: 42 });
      }),
      http.post(`${apiOrigin}/api/workouts/42/sets/102/recommendation`, () => HttpResponse.json(
        { status: 503, detail: "AI提案は一時的に利用できません。" },
        { status: 503, headers: { "Content-Type": "application/problem+json" } },
      )),
    );
    const user = userEvent.setup();
    await startWorkout(user);

    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    expect(await screen.findByText("セットを保存できませんでした")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("80")).toHaveValue(60);
    expect(screen.getByPlaceholderText("80")).toBeDisabled();
    expect(screen.getByPlaceholderText("10")).toHaveValue(8);
    expect(screen.getByPlaceholderText("10")).toBeDisabled();
    expect(screen.getByRole("button", { name: "終了" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "セットの保存を再試行" }));
    expect(await screen.findByText(/セットは保存済みです/)).toBeInTheDocument();
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBeTruthy();
    expect(keys[1]).toBe(keys[0]);
  });

  it("終了集計をAI総評より先に表示する", async () => {
    useCommonHandlers();
    const summaryComment = deferred<Response>();
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/finish`, () => HttpResponse.json({
        workout_id: 42,
        status: "completed",
        summary: {
          total_sets: 1,
          total_reps: 8,
          total_volume: 480,
          duration_min: 4,
          pr_count: 0,
          exercises: [{
            exercise_id: "bench_press",
            name: "ベンチプレス",
            sets: 1,
            total_reps: 8,
            best_weight: 60,
            total_volume: 480,
          }],
        },
      })),
      http.post(`${apiOrigin}/api/workouts/42/summary-comment`, () => summaryComment.promise),
    );
    const user = userEvent.setup();
    await startWorkout(user);

    await user.click(screen.getByRole("button", { name: "終了" }));
    expect(await screen.findByRole("heading", { name: "今日のまとめ" })).toBeInTheDocument();
    expect(screen.getByText("AIコーチが総評を作成中です")).toBeInTheDocument();
    expect(screen.getByText("480")).toBeInTheDocument();

    summaryComment.resolve(HttpResponse.json({ comment: "今日の記録はすでに安全に保存されています。", replayed: false }));
    expect(await screen.findByText("今日の記録はすでに安全に保存されています。")).toBeInTheDocument();
  });

  it("AI調整失敗時に基礎プランで開始したことを表示する", async () => {
    useCommonHandlers("fallback");
    const user = userEvent.setup();
    await startWorkout(user);

    expect(screen.getByText(/AI調整は利用できませんでしたが/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "セットを保存" })).toBeEnabled();
  });

  it("月間プランなしでも選んだ種目を日次予定に保存して開始する", async () => {
    let savedPlan: Record<string, unknown> | null = null;
    let startCalls = 0;
    const manualPlan = {
      ...startedPlan,
      id: 11,
      workout_id: 43,
      ai_status: "not_requested",
      plan: {
        workout_title: "フリーワークアウト",
        target: "自分で選んだ種目",
        estimated_duration_min: 15,
        coach_note: "体調に合わせて無理のない重量で進めましょう。",
        exercises: [{
          exercise_id: "squat",
          name: "スクワット",
          planned_sets: 3,
          target_weight: 0,
          target_reps: 10,
          last_max_weight: 0,
        }],
      },
    };
    server.use(
      http.get(`${apiOrigin}/api/exercises/:exerciseID/settings`, () => HttpResponse.json({ target_sets: 3 })),
      http.post(new RegExp(`${apiOrigin}/api/workout-plans/\\d{4}-\\d{2}-\\d{2}/start`), () => {
        startCalls += 1;
        if (startCalls === 1) {
          return HttpResponse.json(
            { status: 404, detail: "今月の月間プランがまだありません。" },
            { status: 404, headers: { "Content-Type": "application/problem+json" } },
          );
        }
        return HttpResponse.json(manualPlan);
      }),
      http.get(`${apiOrigin}/api/exercises/favorites`, () => HttpResponse.json([])),
      http.get(`${apiOrigin}/api/exercises/recent`, () => HttpResponse.json([])),
      http.get(`${apiOrigin}/api/exercises`, () => HttpResponse.json([
        { id: "squat", name: "スクワット", category: "strength" },
      ])),
      http.put(new RegExp(`${apiOrigin}/api/workout-plans/\\d{4}-\\d{2}-\\d{2}$`), async ({ request }) => {
        savedPlan = await request.json() as Record<string, unknown>;
        return HttpResponse.json({ id: 11, status: "active" });
      }),
    );
    const user = userEvent.setup();
    render(<WorkoutPage />);

    expect(screen.queryByRole("button", { name: "自分で種目を選んで始める" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
    await user.click(screen.getByRole("button", { name: "自分で種目を選んで始める" }));
    await user.type(screen.getByPlaceholderText("種目名で検索..."), "スクワット");
    await user.click(await screen.findByRole("button", { name: /^スクワット/ }));

    await screen.findByRole("heading", { name: "ワークアウト中" });
    expect(startCalls).toBe(2);
    expect(savedPlan).toMatchObject({
      workout_title: "フリーワークアウト",
      exercises: [{ exercise_id: "squat", name: "スクワット" }],
    });
  });

  it("種目を往復しても次のセット番号を保つ", async () => {
    const twoExercisePlan = {
      ...startedPlan,
      plan: {
        ...startedPlan.plan,
        exercises: [
          ...startedPlan.plan.exercises,
          {
            exercise_id: "squat",
            name: "スクワット",
            planned_sets: 3,
            target_weight: 80,
            target_reps: 8,
            last_max_weight: 100,
          },
        ],
      },
    };
    server.use(
      http.get(`${apiOrigin}/api/exercises/:exerciseID/settings`, () => HttpResponse.json({ target_sets: 3 })),
      http.post(new RegExp(`${apiOrigin}/api/workout-plans/\\d{4}-\\d{2}-\\d{2}/start`), () => HttpResponse.json(twoExercisePlan)),
      http.post(`${apiOrigin}/api/workouts/42/sets`, () => HttpResponse.json({ id: 104, workout_id: 42 })),
      http.post(`${apiOrigin}/api/workouts/42/sets/104/recommendation`, () => HttpResponse.json(
        { status: 503, detail: "AI提案は一時的に利用できません。" },
        { status: 503, headers: { "Content-Type": "application/problem+json" } },
      )),
    );
    const user = userEvent.setup();
    await startWorkout(user);

    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    await screen.findByText(/AI提案のみ取得できません/);
    await user.click(screen.getByRole("button", { name: /^スクワット/ }));
    expect(screen.getByText("セット 1 / 3")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^ベンチプレス/ }));
    expect(screen.getByText("セット 2 / 3")).toBeInTheDocument();
  });
});
