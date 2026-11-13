import { expect, type APIRequestContext, type BrowserContext, type Locator, type Page, type Response } from "@playwright/test";

/** 組み込み種目カタログ（backend/tmpkin_jp.json）に実在する種目。名前で検索したとき、名前が前方一致する候補は各1件だけ。 */
export interface CatalogExercise {
  id: string;
  name: string;
}

export const BENCH_PRESS: CatalogExercise = { id: "Barbell_Bench_Press_-_Medium_Grip", name: "バーベルベンチプレス - ミディアムグリップ" };
export const BARBELL_SQUAT: CatalogExercise = { id: "Barbell_Squat", name: "バーベルスクワット" };

/** バックエンドの既定のセッションCookie名（SESSION_COOKIE_NAME）。 */
export const SESSION_COOKIE = "fitlog_session";
const PASSWORD = "smoke-password-1234";

export interface SmokeUser {
  id: number;
  nickname: string;
  password: string;
}

/** 一時DBを使い回しても衝突しないよう、テストごとに新しいニックネームで登録する。 */
function uniqueNickname(label: string) {
  return `smoke-${label}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

/** ブラウザが送ったAPIリクエストの応答を、メソッドとパスで待つ条件。 */
export function apiResponse(method: string, pathname: string) {
  return (response: Response) => response.request().method() === method && new URL(response.url()).pathname === pathname;
}

export function loginHeading(page: Page) {
  return page.getByRole("heading", { name: "Fitlog に入る" });
}

export function greeting(page: Page, nickname: string) {
  return page.getByText(`${nickname}さん`, { exact: true });
}

export async function submitLogin(page: Page, nickname: string, password: string) {
  await expect(loginHeading(page)).toBeVisible();
  await page.getByPlaceholder("例: 筋トレマン", { exact: true }).fill(nickname);
  await page.getByPlaceholder("パスワード", { exact: true }).fill(password);
  const response = page.waitForResponse(apiResponse("POST", "/api/auth/login"));
  await page.getByRole("button", { name: "ログイン", exact: true }).click();
  return response;
}

/** 未登録のニックネームでログインし、新しいユーザーとしてホームに入る。 */
export async function registerUser(page: Page, label: string): Promise<SmokeUser> {
  const nickname = uniqueNickname(label);
  const session = page.waitForResponse(apiResponse("GET", "/api/auth/me"));
  await page.goto("/");
  expect((await session).status(), "未ログインの /api/auth/me は401になる").toBe(401);

  const response = await submitLogin(page, nickname, PASSWORD);
  expect(response.status(), "未登録のニックネームは新規登録になる").toBe(201);
  expect(new URL(response.url()).origin, "ブラウザは同一オリジンの /api を呼び、Next.js がバックエンドへ中継する").toBe(new URL(page.url()).origin);
  const body = await response.json();
  expect(body).toMatchObject({ nickname, created: true });
  expect(body.id).toBeGreaterThan(0);
  await expect(greeting(page, nickname)).toBeVisible();
  await expectSessionCookie(page);
  return { id: body.id, nickname, password: PASSWORD };
}

export async function findSessionCookie(context: BrowserContext) {
  return (await context.cookies()).find(cookie => cookie.name === SESSION_COOKIE);
}

/** ログイン応答の Set-Cookie が中継越しにブラウザへ保存され、JavaScript からは読めないことを確認する。 */
export async function expectSessionCookie(page: Page) {
  const cookie = await findSessionCookie(page.context());
  expect(cookie, "セッションCookieがブラウザに保存されている").toBeDefined();
  expect(cookie).toMatchObject({ domain: new URL(page.url()).hostname, path: "/", httpOnly: true, sameSite: "Lax" });
  expect(await page.evaluate(() => document.cookie)).not.toContain(SESSION_COOKIE);
}

/** 新規ユーザーは当月の月間プランがない（404）ため、ホームで案内が自動表示される。当月は表示しない設定にして閉じる。 */
export async function dismissMonthlyPrompt(page: Page) {
  const dialog = page.getByRole("dialog", { name: "月間プラン" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "今月は次回以降表示しない" }).click();
  await expect(dialog).toBeHidden();
}

/** 画面と同じく、ブラウザのローカル日付（playwright.smoke.config.ts の timezoneId）で YYYY-MM-DD を作る。 */
export async function localDateKey(page: Page, options: { monthOffset?: number; day?: number } = {}) {
  return page.evaluate(({ monthOffset, day }) => {
    const now = new Date();
    const date = new Date(now.getFullYear(), now.getMonth() + monthOffset, day ?? now.getDate());
    const pad = (value: number) => String(value).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  }, { monthOffset: options.monthOffset ?? 0, day: options.day ?? null });
}

function isExerciseSearch(response: Response, name: string | null) {
  const url = new URL(response.url());
  return response.request().method() === "GET" && url.pathname === "/api/exercises" && url.searchParams.get("name") === name;
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * 種目選択モーダルを開き、実際の種目カタログを名前で検索して選ぶ。
 * モーダルは開いた300ms後に絞り込みなしの全件（約1MB）を取得し、古い応答を破棄しない。
 * 遅れて読み終えた全件で検索結果が上書きされないよう、全件の本文を受信し終えてから検索する
 * （応答の待機はヘッダー受信で解決するため、finished() で本文の受信完了まで待つ）。
 */
export async function chooseExercise(page: Page, opener: Locator, exercise: CatalogExercise) {
  const selectorHeading = page.getByRole("heading", { name: "種目を選択" });
  const initialList = page.waitForResponse(response => isExerciseSearch(response, null));
  await opener.click();
  await expect(selectorHeading).toBeVisible();
  const initial = await initialList;
  expect(initial.ok()).toBe(true);
  await initial.finished();

  const searched = page.waitForResponse(response => isExerciseSearch(response, exercise.name));
  await page.getByPlaceholder("種目名で検索...", { exact: true }).fill(exercise.name);
  const results = await searched;
  expect(results.ok()).toBe(true);
  const ids = ((await results.json()) as Array<{ id: string }>).map(item => item.id);
  expect(ids, `組み込み種目 ${exercise.id} を名前で検索できる`).toContain(exercise.id);

  await page.getByRole("button", { name: new RegExp(`^${escapeRegExp(exercise.name)}`) }).click();
  await expect(selectorHeading).toBeHidden();
}

/**
 * compose のスモーク環境ではAIの通信先をスタブに向けている。全テストがAI OFFの操作だけなので、
 * 実行全体で1回も呼ばれていないことを確認する（記録は globalSetup で初期化し、テスト間で共有する）。
 * SMOKE_AI_STUB_URL がない環境（SMOKE_BASE_URL で起動済みの環境に向ける場合）では確認しない。
 */
export async function expectNoAICalls(request: APIRequestContext) {
  const stubURL = process.env.SMOKE_AI_STUB_URL;
  if (!stubURL) return;
  const response = await request.get(new URL("/__stub/calls", stubURL).toString());
  expect(response.ok()).toBe(true);
  expect((await response.json()).calls, "AI OFFのスモーク実行中にAIのスタブが呼ばれた（どのテストの操作かは呼び出し時刻とtraceで確認する）").toEqual([]);
}
