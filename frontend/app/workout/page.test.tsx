import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import WorkoutPage from "./page";
import { AISettingsProvider } from "@/components/AISettings";

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

function renderWorkout(aiEnabled = true) {
  window.localStorage.setItem("fitlog:ai-enabled:1", String(aiEnabled));
  return render(<AISettingsProvider userID={1}><WorkoutPage /></AISettingsProvider>);
}

async function startWorkout(user: ReturnType<typeof userEvent.setup>) {
  renderWorkout();
  await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
  await screen.findByRole("heading", { name: "ワークアウト中" });
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  vi.spyOn(window, "confirm").mockReturnValue(true);
  server.use(http.get(`${apiOrigin}/api/workouts/42`, () => HttpResponse.json({
    id: 42, status: "active",
    summary: { total_sets: 0, total_reps: 0, total_volume: 0, duration_min: 0, pr_count: 0, exercises: [] },
  })));
  server.use(http.get(`${apiOrigin}/api/workouts/by-date/:date`, () => HttpResponse.json({}, { status: 404 })));
});

describe("WorkoutPage record-first flow", () => {
  it("終了確認をキャンセルすると記録を続けられる", async () => {
    useCommonHandlers();
    let finishes = 0;
    server.use(http.post(`${apiOrigin}/api/workouts/42/finish`, () => {
      finishes++;
      return HttpResponse.json({ summary: { total_sets: 0, total_reps: 0, total_volume: 0, duration_min: 1, pr_count: 0, exercises: [] } });
    }));
    const user = userEvent.setup();
    await startWorkout(user);
    vi.mocked(window.confirm).mockReturnValueOnce(false);
    await user.click(screen.getByRole("button", { name: "終了" }));
    expect(window.confirm).toHaveBeenCalledWith("ワークアウトを終了しますか？保存済みのセットで集計します。");
    expect(finishes).toBe(0);
    expect(screen.getByRole("heading", { name: "ワークアウト中" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "セットを保存" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "終了" }));
    await screen.findByRole("heading", { name: "今日のまとめ" });
    expect(finishes).toBe(1);
  });

  it("終了後に新しく始めても同じ記録へ次のセットを追記し、再度終了できる", async () => {
    useCommonHandlers();
    const sets: { exercise_id: string; set_order: number; weight: number; reps: number }[] = [];
    let starts = 0;
    let finishes = 0;
    const summary = () => ({
      total_sets: sets.length, total_reps: sets.length * 8, total_volume: sets.length * 480, duration_min: 4, pr_count: 0,
      exercises: sets.length ? [{ exercise_id: "bench_press", name: "ベンチプレス", sets: sets.length, last_set_order: sets.length, total_reps: sets.length * 8, best_weight: 60, total_volume: sets.length * 480 }] : [],
    });
    server.use(
      http.post(new RegExp(`${apiOrigin}/api/workout-plans/[^/]+/start`), () => {
        starts++;
        return HttpResponse.json({ ...startedPlan, resumed: starts > 1, ai_status: "not_requested" });
      }),
      http.get(`${apiOrigin}/api/workouts/42`, () => HttpResponse.json({ id: 42, status: "active", summary: summary() })),
      http.post(`${apiOrigin}/api/workouts/42/sets`, async ({ request }) => {
        sets.push(await request.json() as typeof sets[number]);
        return HttpResponse.json({ id: 100 + sets.length, workout_id: 42 });
      }),
      http.post(`${apiOrigin}/api/workouts/42/finish`, () => {
        finishes++;
        return HttpResponse.json({ workout_id: 42, status: "completed", summary: summary() });
      }),
    );
    const user = userEvent.setup();
    renderWorkout(false);
    await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
    await screen.findByRole("heading", { name: "ワークアウト中" });
    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    await screen.findByText(/セットは保存済みです/);
    await user.click(screen.getByRole("button", { name: "終了" }));
    await screen.findByRole("heading", { name: "今日のまとめ" });
    expect(screen.getByText("480", { exact: true })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "新しく始める" }));
    await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
    await screen.findByRole("heading", { name: "ワークアウト中" });
    expect(screen.getByText("セット 2 / 3")).toBeInTheDocument();
    expect(screen.getByText(/今日のワークアウトを再開しました/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    await screen.findByText(/セットは保存済みです/);
    await user.click(screen.getByRole("button", { name: "ワークアウト終了" }));
    await screen.findByRole("heading", { name: "今日のまとめ" });
    expect(screen.getByText("960", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("2セット")).toBeInTheDocument();
    expect(sets.map(set => set.set_order)).toEqual([1, 2]);
    expect(starts).toBe(2);
    expect(finishes).toBe(2);
    expect(window.confirm).toHaveBeenCalledTimes(2);
  });

  it("再開済み記録の取得に失敗したら保存画面へ進まず、再試行で選択種目のセット番号を復元する", async () => {
    useCommonHandlers();
    let reads = 0;
    const inputs: unknown[] = [];
    server.use(
      http.get(`${apiOrigin}/api/workouts/42`, () => {
        reads++;
        if (reads === 1) return HttpResponse.json({ detail: "記録を取得できませんでした。" }, { status: 404 });
        return HttpResponse.json({ id: 42, status: "active", summary: {
          total_sets: 2, total_reps: 16, total_volume: 960, duration_min: 4, pr_count: 0,
          exercises: [{ exercise_id: "squat", name: "スクワット", sets: 2, last_set_order: 7, total_reps: 16, best_weight: 60, total_volume: 960 }],
        } });
      }),
      http.post(new RegExp(`${apiOrigin}/api/workout-plans/[^/]+/start`), async ({ request }) => {
        inputs.push(await request.json());
        // 別の開始が先に再開を完了していても、既存セットは復元する。
        return HttpResponse.json({ ...startedPlan, resumed: false, ai_status: "not_requested" });
      }),
      http.get(`${apiOrigin}/api/exercises/favorites`, () => HttpResponse.json([])),
      http.get(`${apiOrigin}/api/exercises/recent`, () => HttpResponse.json([])),
      http.get(`${apiOrigin}/api/exercises`, () => HttpResponse.json([{ id: "squat", name: "スクワット", category: "strength" }])),
    );
    const user = userEvent.setup();
    renderWorkout(false);
    await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
    await screen.findByText("記録を取得できませんでした。");
    expect(screen.queryByRole("button", { name: "セットを保存" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "自分で種目を選んで始める" }));
    await user.type(screen.getByPlaceholderText("種目名で検索..."), "スクワット");
    await user.click(await screen.findByRole("button", { name: /^スクワット/ }));
    await screen.findByRole("heading", { name: "ワークアウト中" });
    expect(screen.getByRole("heading", { name: "スクワット" })).toBeInTheDocument();
    expect(screen.getByText("セット 8 / 3")).toBeInTheDocument();
    expect(inputs).toEqual([{ use_ai: false }, { use_ai: false, exercise_id: "squat" }]);
  });

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

  it("保存待機中のOFFでも保存と入力を維持し、AI提案は開始しない", async () => {
    useCommonHandlers();
    const saved = deferred<Response>();
    let requests = 0;
    let aiCalls = 0;
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/sets`, async () => { requests++; return saved.promise; }),
      http.post(`${apiOrigin}/api/workouts/42/sets/100/recommendation`, () => { aiCalls++; return HttpResponse.json({}); }),
    );
    const user = userEvent.setup();
    await startWorkout(user);
    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    await waitFor(() => expect(requests).toBe(1));
    await user.click(screen.getByRole("switch"));
    await act(async () => { saved.resolve(HttpResponse.json({ id: 100 })); await saved.promise; });
    await screen.findByText(/セットは保存済みです/);
    expect(aiCalls).toBe(0);
    expect(screen.getByPlaceholderText("80")).toHaveValue(60);
    await user.click(screen.getByRole("button", { name: "同じ内容で次へ" }));
    expect(screen.getByText("セット 2 / 3")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("80")).toHaveValue(60);
    await user.click(screen.getByRole("switch"));
    expect(requests).toBe(1);
    expect(aiCalls).toBe(0);
  });

  it("AI提案中にOFF→ONとしても古い提案を適用せず、保存済み状態を保つ", async () => {
    useCommonHandlers();
    const delayed = deferred<Response>();
    let aiCalls = 0;
    let finished = false;
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/sets`, () => HttpResponse.json({ id: 100 })),
      http.post(`${apiOrigin}/api/workouts/42/sets/100/recommendation`, async () => {
        aiCalls++;
        const result = await delayed.promise;
        finished = true;
        return result;
      }),
    );
    const user = userEvent.setup();
    await startWorkout(user);
    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    await screen.findByText("AI提案を取得中");
    await user.click(screen.getByRole("switch"));
    expect(screen.queryByText("AI提案を取得中")).not.toBeInTheDocument();
    expect(screen.getByText(/セットは保存済みです/)).toBeInTheDocument();
    await user.click(screen.getByRole("switch"));
    await act(async () => {
      delayed.resolve(HttpResponse.json({ next_action: "CONTINUE", recommendation: "古い提案", target_weight: 100 }));
      await delayed.promise;
    });
    await waitFor(() => expect(finished).toBe(true));
    expect(screen.queryByText("古い提案")).not.toBeInTheDocument();
    expect(aiCalls).toBe(1);
    expect(screen.getByRole("button", { name: "同じ内容で次へ" })).toBeEnabled();
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

  it("OFFで終了し、ONに戻した後は明示操作だけで総評を取得する", async () => {
    useCommonHandlers();
    let finishes = 0;
    let comments = 0;
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/finish`, () => {
        finishes++;
        return HttpResponse.json({ summary: { total_sets: 0, total_reps: 0, total_volume: 0, duration_min: 1, pr_count: 0, exercises: [] } });
      }),
      http.post(`${apiOrigin}/api/workouts/42/summary-comment`, () => { comments++; return HttpResponse.json({ comment: "後から取得した総評" }); }),
    );
    const user = userEvent.setup();
    renderWorkout(false);
    await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
    await screen.findByRole("heading", { name: "ワークアウト中" });
    await user.click(screen.getByRole("button", { name: "終了" }));
    await screen.findByRole("heading", { name: "今日のまとめ" });
    expect(comments).toBe(0);
    await user.click(screen.getByRole("switch"));
    expect(comments).toBe(0);
    await user.click(screen.getByRole("button", { name: "AI総評を取得" }));
    await screen.findByText("後から取得した総評");
    expect(comments).toBe(1);
    expect(finishes).toBe(1);
  });

  it("総評待機中のOFFは終了結果を維持し、遅い総評を表示しない", async () => {
    useCommonHandlers();
    const delayed = deferred<Response>();
    let finished = false;
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/finish`, () => HttpResponse.json({ summary: { total_sets: 0, total_reps: 0, total_volume: 0, duration_min: 1, pr_count: 0, exercises: [] } })),
      http.post(`${apiOrigin}/api/workouts/42/summary-comment`, async () => { const result = await delayed.promise; finished = true; return result; }),
    );
    const user = userEvent.setup();
    await startWorkout(user);
    await user.click(screen.getByRole("button", { name: "終了" }));
    await screen.findByText("AIコーチが総評を作成中です");
    await user.click(screen.getByRole("switch"));
    expect(screen.queryByText("AIコーチが総評を作成中です")).not.toBeInTheDocument();
    await act(async () => { delayed.resolve(HttpResponse.json({ comment: "遅い総評" })); await delayed.promise; });
    await waitFor(() => expect(finished).toBe(true));
    expect(screen.queryByText("遅い総評")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "今日のまとめ" })).toBeInTheDocument();
  });

  it("終了集計をAI総評より先に表示し、総評の再試行でも記録を再送しない", async () => {
    useCommonHandlers();
    const summaryComment = deferred<Response>();
    const retriedSummaryComment = deferred<Response>();
    let saveCalls = 0;
    let finishCalls = 0;
    let summaryCalls = 0;
    server.use(
      http.post(`${apiOrigin}/api/workouts/42/sets`, () => {
        saveCalls += 1;
        return HttpResponse.json({ id: 105, workout_id: 42 });
      }),
      http.post(`${apiOrigin}/api/workouts/42/sets/105/recommendation`, () => HttpResponse.json({
        next_action: "CONTINUE",
        recommendation: "同じ重量で続けましょう。",
        target_weight: 60,
        target_reps: 8,
        reason: "安定しているため",
        max_weight: 75,
      })),
      http.post(`${apiOrigin}/api/workouts/42/finish`, () => {
        finishCalls += 1;
        return HttpResponse.json({
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
        });
      }),
      http.post(`${apiOrigin}/api/workouts/42/summary-comment`, () => {
        summaryCalls += 1;
        return summaryCalls === 1 ? summaryComment.promise : retriedSummaryComment.promise;
      }),
    );
    const user = userEvent.setup();
    await startWorkout(user);

    await user.click(screen.getByRole("button", { name: "セットを保存" }));
    await screen.findByText("同じ重量で続けましょう。");
    await user.click(screen.getByRole("button", { name: "終了" }));

    expect(await screen.findByRole("heading", { name: "今日のまとめ" })).toBeInTheDocument();
    expect(screen.getByText("AIコーチが総評を作成中です")).toBeInTheDocument();
    expect(screen.getByText("480")).toBeInTheDocument();

    summaryComment.resolve(HttpResponse.json(
      { status: 503, detail: "AI総評が一時的に使えません。" },
      { status: 503, headers: { "Content-Type": "application/problem+json" } },
    ));
    expect(await screen.findByText("AI総評のみ取得できませんでした")).toBeInTheDocument();
    expect(screen.getByText("480")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "AI総評を再試行" }));
    expect(await screen.findByText("AIコーチが総評を作成中です")).toBeInTheDocument();
    expect(screen.getByText("480")).toBeInTheDocument();

    retriedSummaryComment.resolve(HttpResponse.json({ comment: "最後までよく頑張りました。", replayed: false }));
    expect(await screen.findByText("最後までよく頑張りました。")).toBeInTheDocument();
    expect(screen.queryByText("AI総評のみ取得できませんでした")).not.toBeInTheDocument();
    expect(screen.getByText("480")).toBeInTheDocument();
    expect(saveCalls).toBe(1);
    expect(finishCalls).toBe(1);
    expect(summaryCalls).toBe(2);
  });

  it("AI調整失敗時に基礎プランで開始したことを表示する", async () => {
    useCommonHandlers("fallback");
    const user = userEvent.setup();
    await startWorkout(user);

    expect(screen.getByText(/AI調整は利用できませんでしたが/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "セットを保存" })).toBeEnabled();
  });

  it("開始エラーを経由せず、月間プランなしで種目を選んで開始する", async () => {
    const startInputs: unknown[] = [];
    server.use(
      http.get(`${apiOrigin}/api/exercises/:exerciseID/settings`, () => HttpResponse.json({ target_sets: 3 })),
      http.post(new RegExp(`${apiOrigin}/api/workout-plans/[^/]+/start`), async ({ request }) => {
        startInputs.push(await request.json());
        return HttpResponse.json({ ...startedPlan, ai_status: "not_requested" });
      }),
      http.get(`${apiOrigin}/api/exercises/favorites`, () => HttpResponse.json([])),
      http.get(`${apiOrigin}/api/exercises/recent`, () => HttpResponse.json([])),
      http.get(`${apiOrigin}/api/exercises`, () => HttpResponse.json([{ id: "squat", name: "スクワット", category: "strength" }])),
    );
    const user = userEvent.setup();
    renderWorkout(false);
    await user.click(screen.getByRole("button", { name: "自分で種目を選んで始める" }));
    await user.type(screen.getByPlaceholderText("種目名で検索..."), "スクワット");
    await user.click(await screen.findByRole("button", { name: /^スクワット/ }));
    await screen.findByRole("heading", { name: "ワークアウト中" });
    expect(screen.getByRole("heading", { name: "スクワット" })).toBeInTheDocument();
    expect(startInputs).toEqual([{ use_ai: false, exercise_id: "squat" }]);
  });

  it("AI調整を待つ間にOFFにするとAIなしで開始し、古い応答を捨てる", async () => {
    const delayed = deferred<Response>();
    const inputs: unknown[] = [];
    let delayedFinished = false;
    server.use(
      http.get(`${apiOrigin}/api/exercises/:exerciseID/settings`, () => HttpResponse.json({ target_sets: 3 })),
      http.post(new RegExp(`${apiOrigin}/api/workout-plans/[^/]+/start`), async ({ request }) => {
        const input = await request.json() as { use_ai: boolean };
        inputs.push(input);
        if (input.use_ai) { const response = await delayed.promise; delayedFinished = true; return response; }
        return HttpResponse.json({ ...startedPlan, ai_status: "not_requested" });
      }),
    );
    const user = userEvent.setup();
    renderWorkout();
    await user.click(screen.getByRole("button", { name: /ワークアウトを開始/ }));
    await waitFor(() => expect(inputs).toHaveLength(1));
    await user.click(screen.getByRole("switch", { name: "AIを使う" }));
    await screen.findByRole("heading", { name: "ワークアウト中" });
    expect(inputs).toEqual([{ use_ai: true }, { use_ai: false }]);
    await act(async () => {
      delayed.resolve(HttpResponse.json({ ...startedPlan, plan: { ...startedPlan.plan, exercises: [{ ...startedPlan.plan.exercises[0], name: "遅い種目" }] } }));
      await delayed.promise;
    });
    await waitFor(() => expect(delayedFinished).toBe(true));
    expect(screen.queryByText("遅い種目")).not.toBeInTheDocument();
    expect(screen.getByRole("switch")).not.toBeChecked();
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
