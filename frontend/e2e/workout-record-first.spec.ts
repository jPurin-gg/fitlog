import { expect, test } from "@playwright/test";

test("記録はAI障害と分離され、終了後のAI総評も再試行できる", async ({ page }) => {
  let setSaveCalls = 0;
  let summaryCommentCalls = 0;

  await page.route("**/api/**", async route => {
    const request = route.request();
    const url = new URL(request.url());
    const respond = (status: number, body: unknown) => route.fulfill({
      status,
      contentType: status >= 400 ? "application/problem+json" : "application/json",
      body: JSON.stringify(body),
    });

    if (request.method() === "GET" && url.pathname === "/api/auth/me") {
      return respond(200, { id: 1, nickname: "E2E user" });
    }
    if (request.method() === "GET" && /\/api\/exercises\/[^/]+\/settings$/.test(url.pathname)) {
      return respond(200, { target_sets: 3 });
    }
    if (request.method() === "POST" && /\/api\/workout-plans\/\d{4}-\d{2}-\d{2}\/start$/.test(url.pathname)) {
      return respond(200, {
        id: 10,
        workout_id: 42,
        plan_date: "2026-08-24",
        status: "active",
        ai_status: "fallback",
        plan: {
          workout_title: "フォールバックプラン",
          target: "胸",
          estimated_duration_min: 30,
          coach_note: "月間プランの内容で続けます。",
          exercises: [{
            exercise_id: "bench_press",
            name: "ベンチプレス",
            planned_sets: 3,
            target_weight: 60,
            target_reps: 8,
            last_max_weight: 75,
          }],
        },
      });
    }
    if (request.method() === "POST" && url.pathname === "/api/workouts/42/sets") {
      setSaveCalls += 1;
      expect(request.headers()["idempotency-key"]).toBeTruthy();
      return respond(200, { id: 100, workout_id: 42 });
    }
    if (request.method() === "POST" && url.pathname === "/api/workouts/42/sets/100/recommendation") {
      return respond(503, { status: 503, detail: "AI提案は一時的に利用できません。" });
    }
    if (request.method() === "POST" && url.pathname === "/api/workouts/42/finish") {
      return respond(200, {
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
    }
    if (request.method() === "POST" && url.pathname === "/api/workouts/42/summary-comment") {
      summaryCommentCalls += 1;
      if (summaryCommentCalls === 1) {
        return respond(503, { status: 503, detail: "AI総評は一時的に利用できません。" });
      }
      return respond(200, { comment: "記録は保存済みです。無理なく続けましょう。", replayed: false });
    }
    return respond(404, { status: 404, detail: `Unhandled test API: ${request.method()} ${url.pathname}` });
  });

  await page.goto("/workout");
  await page.getByRole("button", { name: /ワークアウトを開始/ }).click();
  await expect(page.getByText(/AI調整は利用できませんでしたが/)).toBeVisible();

  await page.getByRole("button", { name: "セットを保存" }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
  await expect(page.getByText("AI提案のみ取得できませんでした")).toBeVisible();
  expect(setSaveCalls).toBe(1);

  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await expect(page.getByText("セット 2 / 3")).toBeVisible();
  await page.getByRole("button", { name: "終了" }).click();

  await expect(page.getByRole("heading", { name: "今日のまとめ" })).toBeVisible();
  await expect(page.getByText("480", { exact: true })).toBeVisible();
  await expect(page.getByText("AI総評のみ取得できませんでした")).toBeVisible();

  await page.getByRole("button", { name: "AI総評を再試行" }).click();
  await expect(page.getByText("記録は保存済みです。無理なく続けましょう。")).toBeVisible();
  expect(summaryCommentCalls).toBe(2);
});
