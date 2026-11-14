import { expect, test, type Page } from "@playwright/test";
import { BENCH_PRESS, apiResponse, chooseExercise, dismissMonthlyPrompt, expectNoAICalls, registerUser, type CatalogExercise } from "./support";

const startPath = /^\/api\/workout-plans\/\d{4}-\d{2}-\d{2}\/start$/;

async function saveWorkoutSet(page: Page, workoutID: number, set: { exercise: CatalogExercise; setOrder: number; weight: number; reps: number }) {
  await page.getByPlaceholder("80", { exact: true }).fill(String(set.weight));
  await page.getByPlaceholder("10", { exact: true }).fill(String(set.reps));
  const saved = page.waitForResponse(apiResponse("POST", `/api/workouts/${workoutID}/sets`));
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  const response = await saved;
  expect(response.status()).toBe(201);
  // 画面が送った本文が、未知フィールドを拒否するバックエンドの入力契約と一致する。
  expect(response.request().headers()["idempotency-key"]).toMatch(/^[A-Za-z0-9_-]{8,128}$/);
  expect(response.request().postDataJSON()).toEqual({
    exercise_id: set.exercise.id,
    set_order: set.setOrder,
    weight: set.weight,
    reps: set.reps,
    feeling: "",
  });
  expect(await response.json()).toMatchObject({
    id: expect.any(Number),
    workout_id: workoutID,
    exercise_id: set.exercise.id,
    set_order: set.setOrder,
    weight: set.weight,
    reps: set.reps,
    is_pr: expect.any(Boolean),
  });
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
}

test("新規登録から手動開始・セット保存・終了までを実APIで記録し、履歴とカレンダーに反映される", async ({ page, request }) => {
  await registerUser(page, "record");
  await dismissMonthlyPrompt(page);

  await page.goto("/workout");
  await expect(page.getByRole("heading", { name: "今日の筋トレを始めますか？" })).toBeVisible();
  await expect(page.getByRole("switch", { name: "AIを使う" })).toHaveAttribute("aria-checked", "false");

  // 組み込み種目のIDで手動開始する。画面はブラウザのローカル日付をパスに入れ、
  // サーバーは APP_TIMEZONE の当日だけを受け付ける（タイムゾーンがずれていれば400になる）。
  const started = page.waitForResponse(response => response.request().method() === "POST" && startPath.test(new URL(response.url()).pathname));
  await chooseExercise(page, page.getByRole("button", { name: "自分で種目を選んで始める" }), BENCH_PRESS);
  const start = await started;
  expect(start.status()).toBe(200);
  expect(start.request().postDataJSON()).toEqual({ use_ai: false, exercise_id: BENCH_PRESS.id });
  const session = await start.json();
  const today: string = session.plan_date;
  expect(new URL(start.url()).pathname, "画面が送った日付を、サーバーが当日の予定として返す").toBe(`/api/workout-plans/${today}/start`);
  expect(session).toMatchObject({
    ai_status: "not_requested",
    plan: { exercises: [{ exercise_id: BENCH_PRESS.id, name: BENCH_PRESS.name }] },
  });
  const workoutID: number = session.workout_id;
  expect(workoutID).toBeGreaterThan(0);
  await expect(page.getByRole("heading", { name: "ワークアウト中" })).toBeVisible();
  await expect(page.getByRole("heading", { name: BENCH_PRESS.name })).toBeVisible();

  await saveWorkoutSet(page, workoutID, { exercise: BENCH_PRESS, setOrder: 1, weight: 60, reps: 8 });
  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await expect(page.getByText("セット 2 / 3")).toBeVisible();
  await saveWorkoutSet(page, workoutID, { exercise: BENCH_PRESS, setOrder: 2, weight: 65, reps: 5 });

  const finished = page.waitForResponse(apiResponse("POST", `/api/workouts/${workoutID}/finish`));
  await page.getByRole("button", { name: "終了", exact: true }).click();
  const finish = await finished;
  expect(finish.status()).toBe(200);
  expect(await finish.json()).toMatchObject({
    workout_id: workoutID,
    status: "completed",
    summary: {
      total_sets: 2,
      total_reps: 13,
      total_volume: 805,
      exercises: [{ exercise_id: BENCH_PRESS.id, name: BENCH_PRESS.name, sets: 2 }],
    },
  });
  await expect(page.getByRole("heading", { name: "今日のまとめ" })).toBeVisible();
  await expect(page.getByText("805", { exact: true })).toBeVisible();

  // 履歴: ホームの「最近のワークアウト」から、終了したワークアウトの詳細を開ける。
  await page.getByRole("link", { name: "ホームへ" }).click();
  const history = page.getByRole("link", { name: /フリーワークアウトの記録/ });
  await expect(history).toHaveAttribute("href", `/workouts/${workoutID}`);
  await history.click();
  await expect(page).toHaveURL(new RegExp(`/workouts/${workoutID}$`));
  await expect(page.getByRole("heading", { level: 1, name: "フリーワークアウト", exact: true })).toBeVisible();
  await expect(page.getByText("805", { exact: true })).toBeVisible();

  // カレンダー: 今日が実施済みになり、日付の記録に保存したセットが並ぶ。
  await page.getByRole("link", { name: "カレンダー", exact: true }).click();
  const todayCell = page.getByRole("button", { name: `${today} の記録を編集` });
  await expect(todayCell).toContainText("フリーワークアウト");
  await todayCell.click();
  await expect(page.getByRole("heading", { name: "記録を編集" })).toBeVisible();
  const weights = page.getByLabel("重量 kg");
  const reps = page.getByLabel("回数");
  await expect(weights).toHaveCount(2);
  await expect(weights.nth(0)).toHaveValue("60");
  await expect(reps.nth(0)).toHaveValue("8");
  await expect(weights.nth(1)).toHaveValue("65");
  await expect(reps.nth(1)).toHaveValue("5");
  await page.getByRole("button", { name: "キャンセル" }).click();
  await expect(page.getByRole("heading", { name: "記録を編集" })).toBeHidden();

  await expectNoAICalls(request);
});
