import { expect, test, type Page } from "@playwright/test";

type SavedSet = { id: number; exercise_id: string; exercise_name: string; set_order: number; weight: number; reps: number };

async function mockRecordingAPI(page: Page) {
  const saved: SavedSet[] = [];
  const unexpected: string[] = [];
  let started = false;
  let completed = false;
  let authenticated = true;
  const summary = () => ({
    total_sets: saved.length,
    total_reps: saved.reduce((total, set) => total + set.reps, 0),
    total_volume: saved.reduce((total, set) => total + set.weight * set.reps, 0),
    duration_min: 10,
    pr_count: 0,
    exercises: [{ exercise_id: "bench", name: "ベンチプレス", sets: saved.length, total_reps: saved.reduce((total, set) => total + set.reps, 0), best_weight: 60, total_volume: saved.reduce((total, set) => total + set.weight * set.reps, 0) }],
  });

  await page.route("**/api/**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const respond = (status: number, body: unknown) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (method === "GET") {
      if (path === "/api/auth/me") return authenticated
        ? respond(200, { id: 1, nickname: "E2E user" })
        : respond(401, { detail: "ログインしてください。" });
      if (path === "/api/dashboard") return respond(200, { recent_workouts: [] });
      if (path === "/api/preferences") return respond(200, {});
      if (/^\/api\/monthly-plans\//.test(path)) return respond(200, { plan_name: "ブラウザ検証プラン", weekly_routine: [] });
      if (/^\/api\/exercises\/[^/]+\/settings$/.test(path)) return respond(200, { target_sets: 3 });
      if (/^\/api\/workouts\/by-date\//.test(path)) return started
        ? respond(200, { workout_id: 42, date: path.split("/").pop(), title: "今日のプラン", sets: saved })
        : respond(404, { detail: "記録はありません。" });
      if (path === "/api/workouts/42") return respond(200, { id: 42, title: "今日のプラン", status: completed ? "completed" : "active", started_at: new Date().toISOString(), summary: summary() });
    }
    if (method === "POST" && path === "/api/auth/login") {
      authenticated = true;
      return respond(200, { id: 1, nickname: "E2E user", created: false });
    }
    if (method === "POST" && /^\/api\/workout-plans\/[^/]+\/start$/.test(path)) {
      expect(request.postDataJSON()).toEqual({ use_ai: false });
      started = true;
      return respond(200, {
        workout_id: 42, status: "active", ai_status: "not_requested",
        plan: { workout_title: "今日のプラン", target: "胸", estimated_duration_min: 30, exercises: [{ exercise_id: "bench", name: "ベンチプレス", planned_sets: 3, target_weight: 60, target_reps: 8 }] },
      });
    }
    if (method === "POST" && path === "/api/workouts/42/sets") {
      const input = request.postDataJSON();
      expect(request.headers()["idempotency-key"]).toBeTruthy();
      expect(saved.some(set => set.exercise_id === input.exercise_id && set.set_order === input.set_order)).toBe(false);
      const set = { ...input, id: 100 + saved.length, exercise_name: "ベンチプレス" };
      saved.push(set);
      return respond(201, { id: set.id, workout_id: 42 });
    }
    if (method === "POST" && path === "/api/workouts/42/finish") {
      completed = true;
      return respond(200, { workout_id: 42, status: "completed", summary: summary() });
    }
    unexpected.push(`${method} ${path}`);
    return respond(500, { detail: "Unexpected test request" });
  });
  return { saved, unexpected, expireSession: () => { authenticated = false; } };
}

test("2セット保存後のリロードと画面移動で記録を復元し、重複せず続けられる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const api = await mockRecordingAPI(page);
  await page.goto("/workout");
  await page.getByRole("button", { name: "ワークアウトを開始", exact: true }).click();
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await expect(page.getByText("セット 2 / 3")).toBeVisible();
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "記録を再開しますか？" })).toBeVisible();
  await page.getByRole("button", { name: "記録を再開", exact: true }).click();
  await expect(page.getByText("セット 3 / 3")).toBeVisible();
  const records = page.getByRole("region", { name: "今日ここまでの記録" });
  await expect(records.getByText("2セット保存済み")).toBeVisible();
  await expect(records.getByText("1セット目 60kg × 8回")).toBeVisible();
  await expect(records.getByText("2セット目 60kg × 8回")).toBeVisible();
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(records.getByText("3セット保存済み")).toBeVisible();
  expect(api.saved.map(set => set.set_order)).toEqual([1, 2, 3]);

  await page.getByRole("link", { name: "ホーム", exact: true }).click();
  await expect(page.getByRole("heading", { name: "記録の途中です" })).toBeVisible();
  await page.getByRole("link", { name: "記録を再開", exact: true }).first().click();
  await page.getByRole("button", { name: "記録を再開", exact: true }).click();
  await expect(page.getByText("セット 4 / 3")).toBeVisible();
  await page.getByRole("button", { name: "終了", exact: true }).click();
  await expect(page.getByRole("heading", { name: "今日のまとめ" })).toBeVisible();

  await page.getByRole("link", { name: "ホーム", exact: true }).click();
  const resultLinks = page.getByRole("link", { name: "今日の結果を見る" });
  await expect(resultLinks).toHaveCount(2);
  for (const link of await resultLinks.all()) await expect(link).toHaveAttribute("href", "/workouts/42");
  await page.getByRole("link", { name: "記録する", exact: true }).click();
  await expect(page.getByRole("link", { name: "今日の結果を見る" })).toHaveAttribute("href", "/workouts/42");
  await expect(page.getByRole("button", { name: "ワークアウトを開始", exact: true })).not.toBeVisible();
  await expect(page.getByRole("button", { name: "自分で種目を選んで始める", exact: true })).not.toBeVisible();
  expect(api.unexpected).toEqual([]);
});

test("未保存入力があるときはリンク移動とリロードを確認し、キャンセルで入力を保つ", async ({ page }) => {
  const api = await mockRecordingAPI(page);
  await page.goto("/workout");
  await page.getByRole("button", { name: "ワークアウトを開始", exact: true }).click();
  await page.getByPlaceholder("80", { exact: true }).fill("65");
  const linkDialog = page.waitForEvent("dialog");
  const clickHome = page.getByRole("link", { name: "ホーム", exact: true }).click();
  const confirmation = await linkDialog;
  expect(confirmation.type()).toBe("confirm");
  expect(confirmation.message()).toContain("保存していない入力があります");
  await confirmation.dismiss();
  await clickHome;
  await expect(page).toHaveURL(/\/workout$/);
  await expect(page.getByPlaceholder("80", { exact: true })).toHaveValue("65");

  const reloadDialog = page.waitForEvent("dialog");
  const reload = page.evaluate(() => window.location.reload());
  const unload = await reloadDialog;
  expect(unload.type()).toBe("beforeunload");
  await unload.dismiss();
  await reload;
  await expect(page.getByPlaceholder("80", { exact: true })).toHaveValue("65");

  page.once("dialog", dialog => dialog.accept());
  await page.getByRole("link", { name: "ホーム", exact: true }).click();
  await expect(page.getByRole("heading", { name: "記録の途中です" })).toBeVisible();
  expect(api.saved).toEqual([]);
  expect(api.unexpected).toEqual([]);
});

test("セッション切れから同じユーザーで再ログインしても保存済み2セットを復元できる", async ({ page }) => {
  const api = await mockRecordingAPI(page);
  await page.goto("/workout");
  await page.getByRole("button", { name: "ワークアウトを開始", exact: true }).click();
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();

  api.expireSession();
  await page.reload();
  await expect(page.getByRole("heading", { name: "Fitlog に入る" })).toBeVisible();
  await page.getByPlaceholder("例: 筋トレマン").fill("E2E user");
  await page.getByPlaceholder("パスワード", { exact: true }).fill("test-only-password");
  await page.getByRole("button", { name: "ログイン", exact: true }).click();
  await page.getByRole("button", { name: "記録を再開", exact: true }).click();
  await expect(page.getByText("セット 3 / 3")).toBeVisible();
  const records = page.getByRole("region", { name: "今日ここまでの記録" });
  await expect(records.getByText("2セット保存済み")).toBeVisible();
  await expect(records.getByText("1セット目 60kg × 8回")).toBeVisible();
  await expect(records.getByText("2セット目 60kg × 8回")).toBeVisible();
  expect(api.saved.map(set => set.set_order)).toEqual([1, 2]);
  expect(api.unexpected).toEqual([]);
});
