import { expect, test, type Page, type Request, type Route } from "@playwright/test";

const startedPlan = {
  id: 10,
  workout_id: 42,
  plan_date: "2026-08-24",
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
      last_max_weight: 75,
    }],
  },
};

const summary = {
  total_sets: 1,
  total_reps: 8,
  total_volume: 480,
  duration_min: 4,
  pr_count: 0,
  exercises: [{ exercise_id: "bench_press", name: "ベンチプレス", sets: 1, total_reps: 8, best_weight: 60, total_volume: 480 }],
};

// ブラウザ側の待ち時間（lib/api.ts の apiTimeouts.standardMs = 15秒）を確実に超える時間。
const pastStandardTimeoutMs = 16_000;

function isRequest(request: Request, method: string, pathname: string) {
  return request.method() === method && new URL(request.url()).pathname === pathname;
}

function respond(route: Route, status: number, body: unknown) {
  return route.fulfill({
    status,
    contentType: status >= 400 ? "application/problem+json" : "application/json",
    body: JSON.stringify(body),
  });
}

// overrides が true を返したリクエストは処理済みとして扱い、それ以外は正常系の応答を返す。
async function mockAPI(page: Page, overrides: (route: Route, request: Request) => boolean) {
  await page.route("**/api/**", async route => {
    const request = route.request();
    if (overrides(route, request)) return;
    if (isRequest(request, "GET", "/api/auth/me")) return respond(route, 200, { id: 1, nickname: "E2E user" });
    if (request.method() === "GET" && /\/api\/exercises\/[^/]+\/settings$/.test(new URL(request.url()).pathname)) {
      return respond(route, 200, { target_sets: 3 });
    }
    if (request.method() === "POST" && /\/api\/workout-plans\/\d{4}-\d{2}-\d{2}\/start$/.test(new URL(request.url()).pathname)) {
      return respond(route, 200, startedPlan);
    }
    if (isRequest(request, "POST", "/api/workouts/42/sets")) return respond(route, 201, { id: 100, workout_id: 42 });
    if (isRequest(request, "POST", "/api/workouts/42/finish")) return respond(route, 200, { workout_id: 42, status: "completed", summary });
    return respond(route, 404, { status: 404, detail: `Unhandled test API: ${request.method()} ${request.url()}` });
  });
}

function collectSetKeys(page: Page) {
  const keys: string[] = [];
  page.on("request", request => {
    if (isRequest(request, "POST", "/api/workouts/42/sets")) keys.push(request.headers()["idempotency-key"] || "");
  });
  return keys;
}

test("応答しないAPIでも、起動・保存・終了が一定時間で再試行できる状態になる", async ({ page }) => {
  // それぞれ最初の1回だけ応答を保留し、サーバーが返事をしない状態を再現する。
  const held = new Set(["/api/auth/me", "/api/workouts/42/sets", "/api/workouts/42/finish"]);
  await mockAPI(page, (_route, request) => held.delete(new URL(request.url()).pathname));
  const setKeys = collectSetKeys(page);
  await page.clock.install();

  // 起動: セッション確認が返らなくても、スピナーのまま止まらない。
  const sessionRequest = page.waitForRequest(request => isRequest(request, "GET", "/api/auth/me"));
  await page.goto("/workout");
  await sessionRequest;
  await page.clock.fastForward(pastStandardTimeoutMs);
  await expect(page.getByText("ログイン状態を確認できませんでした")).toBeVisible();
  await expect(page.getByText(/サーバーからの応答がありませんでした/)).toBeVisible();
  await page.getByRole("button", { name: "ログイン状態を再確認" }).click();

  await page.getByRole("button", { name: /ワークアウトを開始/ }).click();
  await expect(page.getByRole("heading", { name: "ワークアウト中" })).toBeVisible();

  // 保存: 「保存中...」のまま止まらず、同じ冪等キーの再送で1件だけ保存される。
  const setRequest = page.waitForRequest(request => isRequest(request, "POST", "/api/workouts/42/sets"));
  await page.getByRole("button", { name: "セットを保存" }).click();
  await setRequest;
  await page.clock.fastForward(pastStandardTimeoutMs);
  await expect(page.getByText("セットを保存できませんでした")).toBeVisible();
  await expect(page.getByText(/自動で再送します/)).toBeVisible();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible({ timeout: 15_000 });
  expect(setKeys).toHaveLength(2);
  expect(setKeys[0]).toBeTruthy();
  expect(setKeys[1]).toBe(setKeys[0]);

  // 終了: 「終了中...」のまま止まらず、もう一度終了できる。
  const finishRequest = page.waitForRequest(request => isRequest(request, "POST", "/api/workouts/42/finish"));
  await page.getByRole("button", { name: "終了", exact: true }).click();
  await finishRequest;
  await page.clock.fastForward(pastStandardTimeoutMs);
  await expect(page.getByText("ワークアウトを終了できませんでした")).toBeVisible();
  await expect(page.getByText(/サーバーからの応答がありませんでした/)).toBeVisible();
  await page.getByRole("button", { name: "もう一度試す" }).click();
  await expect(page.getByRole("heading", { name: "今日のまとめ" })).toBeVisible();
  await expect(page.getByText("480", { exact: true })).toBeVisible();
});

test("オフラインから復帰すると、固定中のセットを同じ冪等キーで自動再送する", async ({ page, context }) => {
  let offline = false;
  await mockAPI(page, (route, request) => {
    if (!offline || !isRequest(request, "POST", "/api/workouts/42/sets")) return false;
    void route.abort("internetdisconnected");
    return true;
  });
  const setKeys = collectSetKeys(page);

  await page.goto("/workout");
  await page.getByRole("button", { name: /ワークアウトを開始/ }).click();
  await expect(page.getByRole("heading", { name: "ワークアウト中" })).toBeVisible();

  offline = true;
  await context.setOffline(true);
  await expect(page.getByText(/オフラインです。入力内容はこの画面に残ります/)).toBeVisible();

  await page.getByRole("button", { name: "セットを保存" }).click();
  await expect(page.getByText("セットを保存できませんでした")).toBeVisible();
  await expect(page.getByText("オフラインです。回線が戻ると自動で再送します。")).toBeVisible();
  await expect(page.getByPlaceholder("80")).toBeDisabled();
  await expect(page.getByRole("button", { name: "終了", exact: true })).toBeDisabled();

  offline = false;
  await context.setOffline(false);

  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
  await expect(page.getByText(/オフラインです/)).toHaveCount(0);
  expect(setKeys).toHaveLength(2);
  expect(setKeys[0]).toBeTruthy();
  expect(setKeys[1]).toBe(setKeys[0]);
});
