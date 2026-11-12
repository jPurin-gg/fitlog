import { expect, test, type Page } from "@playwright/test";

import { expectNoSeriousViolations } from "./a11y";

type SavedSet = { exercise_id: string; set_order: number; weight: number; reps: number };
const exercises = [
  { id: "bench", name: "ベンチプレス", category: "strength", equipment: "バーベル", primaryMuscles: ["大胸筋"] },
  { id: "squat", name: "スクワット", category: "strength", equipment: "バーベル", primaryMuscles: ["大腿四頭筋"] },
];

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
      if (path === "/api/calendar") return respond(200, { worked_out_dates: [], worked_out_days: [] });
      if (path === "/api/monthly-plans") return respond(200, []);
      if (path.startsWith("/api/workouts/by-date/")) return respond(200, {
        workout_id: 42, title: "胸の日", sets: [{ exercise_id: "bench", exercise_name: "ベンチプレス", weight: 60, reps: 8, set_order: 1, feeling: "余裕" }],
      });
      if (path === "/api/workouts/42") return respond(200, {
        id: 42, title: "胸の日", status: "completed", started_at: "2026-11-01T10:00:00Z", ended_at: "2026-11-01T10:30:00Z",
        summary: { total_sets: 1, total_reps: 8, total_volume: 480, duration_min: 30, pr_count: 1,
          exercises: [{ exercise_id: "bench", name: "ベンチプレス", sets: 1, total_reps: 8, best_weight: 60, total_volume: 480 }] },
      });
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
  const selector = page.getByRole("dialog", { name: "種目を選択" });
  await expect(selector.getByRole("button", { name: "閉じる" })).toBeFocused();
  await expectNoSeriousViolations(page);
  await page.getByRole("button", { name: /^ベンチプレス/ }).click();
  await expect(page.getByRole("heading", { name: "ワークアウト中" })).toBeVisible();
  await expectNoSeriousViolations(page);
  await page.getByRole("spinbutton", { name: "重量（kg）" }).fill("0");
  await page.getByRole("spinbutton", { name: "回数", exact: true }).fill("12");
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await expect(page.getByText("セット 2 / 3")).toBeVisible();

  await page.getByRole("spinbutton", { name: "重量（kg）" }).fill("5");
  await page.getByRole("spinbutton", { name: "回数", exact: true }).fill("8");
  await page.getByRole("switch").click();
  await page.getByRole("switch").click();
  await expect(page.getByRole("spinbutton", { name: "重量（kg）" })).toHaveValue("5");
  await expect(page.getByRole("spinbutton", { name: "回数", exact: true })).toHaveValue("8");
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await page.getByRole("button", { name: "同じ内容で次へ" }).click();
  await page.getByRole("button", { name: "種目を検索して変更" }).click();
  await page.getByRole("button", { name: /^スクワット/ }).click();
  await page.getByRole("spinbutton", { name: "重量（kg）" }).fill("20");
  await page.getByRole("spinbutton", { name: "回数", exact: true }).fill("8");
  await page.getByRole("button", { name: "セットを保存", exact: true }).click();
  await expect(page.getByText(/セットは保存済みです/)).toBeVisible();
  await page.getByRole("button", { name: "終了", exact: true }).click();
  await expect(page.getByRole("heading", { name: "今日のまとめ" })).toBeVisible();
  await expect(page.getByText("200", { exact: true })).toBeVisible();
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
  const api = await mockAPI(page);
  await page.goto("/");
  const dialog = page.getByRole("dialog", { name: "月間プラン" });
  await expect(dialog).toBeVisible();
  await expectNoSeriousViolations(page);
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


test("設定モーダルでフォーカスを循環し、Escapeで開いたボタンに戻る", async ({ page }) => {
  await mockAPI(page);
  await page.goto("/");
  await page.getByRole("dialog", { name: "月間プラン" }).getByRole("button", { name: "閉じる" }).click();
  const trigger = page.getByRole("button", { name: "トレーニング設定" });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "トレーニング設定" });
  const close = dialog.getByRole("button", { name: "閉じる" });
  await expect(close).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(dialog.getByRole("button", { name: "保存する" })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(close).toBeFocused();
  await expectNoSeriousViolations(page);
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  await expect(trigger).toBeFocused();
});

test("ログインフォームをラベルで操作でき、axeの重大な違反がない", async ({ page }) => {
  await mockAPI(page);
  await page.route("**/api/auth/me", route => route.fulfill({ status: 401, contentType: "application/json", body: "{}" }));
  await page.route("**/api/auth/login", route => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ id: 1, nickname: "E2E user" }) }));
  await page.goto("/workout");
  await expect(page.getByRole("heading", { name: "Fitlog に入る" })).toBeVisible();
  await expectNoSeriousViolations(page);
  await page.getByRole("textbox", { name: "ニックネーム" }).fill("E2E user");
  await page.getByLabel("パスワード").fill("test-password");
  await page.getByRole("button", { name: "ログイン" }).click();
  await expect(page.getByRole("button", { name: "自分で種目を選んで始める" })).toBeVisible();
});


test("カレンダーで種目選択を重ねて閉じ、編集から履歴詳細へ進める", async ({ page }) => {
  const api = await mockAPI(page);
  await page.clock.install({ time: new Date("2026-11-15T12:00:00Z") });
  await page.goto("/calendar");
  await page.getByRole("button", { name: "2026-11-01 の記録を編集" }).click();
  const editor = page.getByRole("dialog", { name: "記録を編集" });
  await expect(editor.getByRole("textbox", { name: "タイトル" })).toHaveValue("胸の日");
  await expectNoSeriousViolations(page);
  const trigger = editor.getByRole("button", { name: "種目", exact: true });
  await trigger.click();
  const selector = page.getByRole("dialog", { name: "種目を選択" });
  await expect(selector).toBeVisible();
  await expect(selector.getByRole("button", { name: "閉じる" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(selector).not.toBeVisible();
  await expect(editor).toBeVisible();
  await expect(trigger).toBeFocused();
  await editor.getByRole("link", { name: "詳細" }).click();
  await expect(page.getByRole("heading", { name: "胸の日" })).toBeVisible();
  await expectNoSeriousViolations(page);
  expect(api.unexpected).toEqual([]);
});

test("種目辞書の詳細と追加モーダルを操作できる", async ({ page }) => {
  const api = await mockAPI(page);
  await page.goto("/exercises");
  await page.getByRole("button", { name: "詳細を見る" }).first().click();
  const details = page.getByRole("dialog", { name: "ベンチプレスの詳細" });
  await expect(details).toBeVisible();
  await expectNoSeriousViolations(page);
  await page.keyboard.press("Escape");
  const trigger = page.getByRole("button", { name: "種目を追加" });
  await trigger.click();
  const form = page.getByRole("dialog", { name: "種目を追加" });
  await expect(form.getByRole("textbox", { name: "種目名 *", exact: true })).toBeVisible();
  await expectNoSeriousViolations(page);
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  expect(api.unexpected).toEqual([]);
});
