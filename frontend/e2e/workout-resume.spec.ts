import { expect, test, type Page } from "@playwright/test";

type SavedSet = { id: number; exercise_id: string; exercise_name: string; set_order: number; weight: number; reps: number; feeling: string };

async function mockResumeAPI(page: Page, completed = false) {
  const saved: SavedSet[] = [1, 2].map(order => ({ id: order, exercise_id: "bench", exercise_name: "ベンチプレス", set_order: order, weight: 60, reps: 8, feeling: "" }));
  let status = completed ? "completed" : "active";
  let starts = 0;
  const unexpected: string[] = [];
  const keys = new Set<string>();
  const summary = () => ({
    total_sets: saved.length, total_reps: saved.length * 8, total_volume: saved.length * 480, duration_min: 10, pr_count: 0,
    exercises: [{ exercise_id: "bench", name: "ベンチプレス", sets: saved.length, last_set_order: Math.max(...saved.map(set => set.set_order)), total_reps: saved.length * 8, best_weight: 60, total_volume: saved.length * 480 }],
  });
  await page.route("**/api/**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const respond = (code: number, body: unknown) => route.fulfill({ status: code, contentType: "application/json", body: JSON.stringify(body) });
    if ((method === "GET" && path === "/api/auth/me") || (method === "POST" && path === "/api/auth/login")) return respond(200, { id: 1, nickname: "E2E user" });
    if (method === "GET") {
      if (/^\/api\/workouts\/by-date\//.test(path)) return respond(200, { workout_id: 42, sets: saved });
      if (path === "/api/workouts/42") return respond(200, { id: 42, status, summary: summary() });
      if (/^\/api\/exercises\/[^/]+\/settings$/.test(path)) return respond(200, { target_sets: 3 });
      if (path === "/api/dashboard") return respond(200, { recent_workouts: [] });
      if (path === "/api/preferences") return respond(200, {});
      // 休息日でも進行中・完了済み記録の再開導線を維持する。
      if (/^\/api\/monthly-plans\//.test(path)) return respond(200, { recommended_days: [], weekly_routine: [] });
    }
    if (method === "POST" && /^\/api\/workout-plans\/[^/]+\/start$/.test(path)) {
      expect(request.postDataJSON()).toEqual({ use_ai: false });
      const resumed = status === "completed";
      status = "active";
      starts++;
      return respond(200, { id: 10, workout_id: 42, plan_date: new Date().toISOString().slice(0, 10), status, ai_status: "not_requested", resumed,
        plan: { workout_title: "今日のプラン", target: "胸", estimated_duration_min: 30, coach_note: "",
          exercises: [{ exercise_id: "bench", name: "ベンチプレス", planned_sets: 3, target_weight: 60, target_reps: 8 }] } });
    }
    if (method === "POST" && path === "/api/workouts/42/sets") {
      const key = request.headers()["idempotency-key"];
      expect(key).toBeTruthy();
      expect(keys.has(key)).toBe(false);
      keys.add(key);
      const input = request.postDataJSON();
      expect(saved.some(set => set.exercise_id === input.exercise_id && set.set_order === input.set_order)).toBe(false);
      const id = 100 + saved.length;
      saved.push({ ...input, id, exercise_name: "ベンチプレス" });
      return respond(201, { id });
    }
    unexpected.push(`${method} ${path}`);
    return respond(404, { detail: "Unexpected test request" });
  });
  return { saved, unexpected, starts: () => starts };
}

test("リロード・ホーム移動・再ログイン後も保存済み2セットから再開できる", async ({ page }) => {
  const api = await mockResumeAPI(page);
  const history = page.getByRole("region", { name: "今日ここまでの記録" });
  await page.goto("/workout");
  expect(api.starts()).toBe(0); // 表示だけで開始APIを呼ばない。
  await page.getByRole("button", { name: "ワークアウトを再開", exact: true }).click();
  await expect(page.getByText("セット 3 / 3")).toBeVisible();
  await expect(history.getByRole("listitem")).toHaveCount(2);
  await page.reload();
  await page.getByRole("button", { name: "ワークアウトを再開", exact: true }).click();
  await expect(page.getByText("セット 3 / 3")).toBeVisible();
  await expect(history.getByRole("listitem")).toHaveCount(2);
  await page.getByRole("link", { name: "ホーム", exact: true }).click();
  await page.getByRole("link", { name: "記録を再開", exact: true }).click();
  await page.getByRole("button", { name: "ワークアウトを再開", exact: true }).click();
  await expect(page.getByText("セット 3 / 3")).toBeVisible();
  await page.evaluate(() => window.dispatchEvent(new Event("fitlog:unauthorized")));
  await expect(page.getByRole("heading", { name: "Fitlog に入る" })).toBeVisible();
  await page.getByPlaceholder("例: 筋トレマン").fill("E2E user");
  await page.getByPlaceholder("パスワード", { exact: true }).fill("mock-password");
  await page.getByRole("button", { name: "ログイン", exact: true }).click();
  await page.getByRole("button", { name: "ワークアウトを再開", exact: true }).click();
  await expect(page.getByText("セット 3 / 3")).toBeVisible();
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(history.getByRole("listitem")).toHaveCount(3);
  expect(api.saved.map(set => set.set_order)).toEqual([1, 2, 3]);
  expect(api.unexpected).toEqual([]);
});

test("完了済みの日も再開して追記でき、未保存入力のナビ離脱をキャンセルできる", async ({ page }) => {
  const api = await mockResumeAPI(page, true);
  await page.goto("/");
  await expect(page.getByRole("link", { name: "今日の記録を見る" })).toHaveAttribute("href", "/workouts/42");
  await page.getByRole("link", { name: "記録を再開して追記", exact: true }).click();
  await page.getByRole("button", { name: "ワークアウトを再開して追記", exact: true }).click();
  await expect(page.getByText("セット 3 / 3")).toBeVisible();
  await page.getByPlaceholder("80", { exact: true }).fill("65");
  page.once("dialog", dialog => dialog.dismiss());
  await page.getByRole("link", { name: "ホーム", exact: true }).click();
  await expect(page.getByRole("heading", { name: "ワークアウト中" })).toBeVisible();
  await expect(page.getByPlaceholder("80", { exact: true })).toHaveValue("65");
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
  await page.getByRole("link", { name: "ホーム", exact: true }).click();
  await expect(page.getByRole("link", { name: "記録を再開", exact: true })).toBeVisible();
  expect(api.saved.map(set => set.set_order)).toEqual([1, 2, 3]);
  expect(api.unexpected).toEqual([]);
});
