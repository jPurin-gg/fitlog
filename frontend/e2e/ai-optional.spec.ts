import { expect, test, type Page } from "@playwright/test";

type SavedSet = { exercise_id: string; set_order: number; weight: number; reps: number };
const exercises = [{ id: "bench", name: "ベンチプレス" }, { id: "squat", name: "スクワット" }];

async function mockAPI(page: Page) {
  const saved: SavedSet[] = [];
  const starts: unknown[] = [];
  const aiCalls: string[] = [];
  const unexpected: string[] = [];
  const keys = new Set<string>();
  let finishes = 0;
  await page.route("**/api/**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const respond = (status: number, body: unknown) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (method === "GET") {
      if (path === "/api/auth/me") return respond(200, { id: 1, nickname: "E2E user" });
      if (path === "/api/dashboard") return respond(200, { recent_workouts: [] });
      if (path === "/api/preferences") return respond(200, {});
      if (/^\/api\/monthly-plans\/\d{4}-\d{2}$/.test(path)) return respond(404, { detail: "月間プランはありません。" });
      if (["/api/exercises", "/api/exercises/favorites"].includes(path)) return respond(200, exercises);
      if (path === "/api/exercises/recent") return respond(200, []);
      if (/^\/api\/exercises\/[^/]+\/settings$/.test(path)) return respond(200, { target_sets: 3 });
    }
    if (method === "POST" && /^\/api\/workout-plans\/[^/]+\/start$/.test(path)) {
      const input = request.postDataJSON();
      starts.push(input);
      expect(input).toEqual({ use_ai: false, exercise_id: "bench" });
      return respond(200, {
        workout_id: 42, status: "active", ai_status: "not_requested",
        plan: {
          workout_title: "フリーワークアウト", target: "自分で選んだ種目", estimated_duration_min: 15,
          exercises: [{ exercise_id: "bench", name: "ベンチプレス", planned_sets: 3, target_weight: 0, target_reps: 10, last_max_weight: 0 }],
        },
      });
    }
    if (method === "POST" && path === "/api/workouts/42/sets") {
      const key = request.headers()["idempotency-key"];
      expect(key).toBeTruthy();
      expect(keys.has(key)).toBe(false);
      keys.add(key);
      saved.push(request.postDataJSON());
      return respond(201, { id: 100 + saved.length, workout_id: 42 });
    }
    if (method === "POST" && path === "/api/workouts/42/finish") {
      finishes++;
      const summary = {
        total_sets: saved.length, total_reps: saved.reduce((n, set) => n + set.reps, 0),
        total_volume: saved.reduce((n, set) => n + set.weight * set.reps, 0), duration_min: 5, pr_count: 0,
        exercises: exercises.map(exercise => {
          const sets = saved.filter(set => set.exercise_id === exercise.id);
          return { exercise_id: exercise.id, name: exercise.name, sets: sets.length, total_reps: sets.reduce((n, set) => n + set.reps, 0), best_weight: Math.max(0, ...sets.map(set => set.weight)), total_volume: sets.reduce((n, set) => n + set.weight * set.reps, 0) };
        }).filter(exercise => exercise.sets > 0),
      };
      return respond(200, { workout_id: 42, status: "completed", summary });
    }
    if (/\/(generate|recommendation|summary-comment|alternatives)$/.test(path)) {
      aiCalls.push(path);
      return respond(503, { detail: "AI unavailable" });
    }
    unexpected.push(`${method} ${path}`);
    return respond(404, { detail: "Unexpected test request" });
  });
  return { saved, starts, aiCalls, unexpected, finishes: () => finishes };
}

test("月間プランなし・AI OFFで複数セットと種目変更を記録し、終了できる", async ({ page }, testInfo) => {
  const api = await mockAPI(page);
  await page.goto("/workout");
  await expect(page.getByRole("switch", { name: "AIを使う" })).not.toBeChecked();
  await page.getByRole("button", { name: "自分で種目を選んで始める" }).click();
  await page.getByRole("button", { name: "ベンチプレス", exact: true }).click();
  await expect(page.getByRole("heading", { name: "ワークアウト中" })).toBeVisible();
  await page.getByPlaceholder("80", { exact: true }).fill("0");
  await page.getByPlaceholder("10", { exact: true }).fill("12");
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await expect(page.getByText("セット 2 / 3")).toBeVisible();

  await page.getByPlaceholder("80", { exact: true }).fill("5");
  await page.getByPlaceholder("10", { exact: true }).fill("8");
  await page.getByRole("switch").click();
  await page.getByRole("switch").click();
  await expect(page.getByPlaceholder("80", { exact: true })).toHaveValue("5");
  await expect(page.getByPlaceholder("10", { exact: true })).toHaveValue("8");
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await page.getByRole("button", { name: "種目を検索して変更" }).click();
  await page.getByRole("button", { name: "スクワット", exact: true }).click();
  await page.getByPlaceholder("80", { exact: true }).fill("20");
  await page.getByPlaceholder("10", { exact: true }).fill("8");
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
  await page.getByRole("button", { name: "終了", exact: true }).click();
  await expect(page.getByRole("heading", { name: "今日のまとめ" })).toBeVisible();
  // Temporary #37 validation: exercise failure reports and retry traces in Actions.
  await expect(page.getByText("201", { exact: true })).toBeVisible();
  await page.getByRole("switch").click();
  await expect(page.getByRole("button", { name: "AI総評を取得", exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("manual-summary.png"), fullPage: true });
  expect(api.saved.map(({ exercise_id, set_order, weight, reps }) => ({ exercise_id, set_order, weight, reps }))).toEqual([
    { exercise_id: "bench", set_order: 1, weight: 0, reps: 12 },
    { exercise_id: "bench", set_order: 2, weight: 5, reps: 8 },
    { exercise_id: "squat", set_order: 1, weight: 20, reps: 8 },
  ]);
  expect(api.starts).toHaveLength(1);
  expect(api.finishes()).toBe(1);
  expect(api.aiCalls).toEqual([]);
  expect(api.unexpected).toEqual([]);
});

test("今月の案内を非表示にして画面移動・再読込し、手動で開き直せる", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const api = await mockAPI(page);
  await page.goto("/");
  const dialog = page.getByRole("dialog", { name: "月間プラン" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("switch")).not.toBeChecked();
  await expect(dialog.getByRole("link", { name: "プランなしで記録を始める" })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("monthly-prompt-mobile.png"), fullPage: true });
  await dialog.getByRole("button", { name: "今月は次回以降表示しない" }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole("link", { name: "プランなしで記録を始める" }).click();
  await expect(page.getByRole("button", { name: "自分で種目を選んで始める" })).toBeVisible();
  await expect(dialog).not.toBeVisible();
  await page.getByRole("switch").click();
  await page.goto("/");
  await expect(page.getByText("まだ完了したワークアウトはありません。")).toBeVisible();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole("switch")).toBeChecked();
  await page.getByRole("button", { name: "月間プランを作成／見直す" }).click();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("switch")).toBeChecked();
  await dialog.getByRole("button", { name: "閉じる" }).click();
  await page.reload();
  await expect(page.getByText("まだ完了したワークアウトはありません。")).toBeVisible();
  await expect(dialog).not.toBeVisible();
  expect(api.aiCalls).toEqual([]);
  expect(api.unexpected).toEqual([]);
});
