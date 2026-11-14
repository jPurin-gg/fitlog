import { expect, test } from "@playwright/test";
import {
  SESSION_COOKIE,
  apiResponse,
  dismissMonthlyPrompt,
  expectNoAICalls,
  expectSessionCookie,
  findSessionCookie,
  greeting,
  loginHeading,
  registerUser,
  submitLogin,
} from "./support";

test("ログアウト後は誤ったパスワードを拒否し、正しいパスワードで同じユーザーに再ログインできる", async ({ page, context, request }) => {
  const user = await registerUser(page, "relogin");
  await dismissMonthlyPrompt(page);

  // 再読み込みしても、中継越しに保存したCookieでセッションを復元する。
  const restored = page.waitForResponse(apiResponse("GET", "/api/auth/me"));
  await page.reload();
  expect((await restored).status()).toBe(200);
  await expect(greeting(page, user.nickname)).toBeVisible();

  const loggedOut = page.waitForResponse(apiResponse("DELETE", "/api/auth/session"));
  await page.getByRole("button", { name: "ログアウト" }).click();
  expect((await loggedOut).status()).toBe(204);
  await expect(loginHeading(page)).toBeVisible();
  expect(await findSessionCookie(context), "ログアウト応答でCookieが削除される").toBeUndefined();

  const anonymous = page.waitForResponse(apiResponse("GET", "/api/auth/me"));
  await page.reload();
  expect((await anonymous).status()).toBe(401);
  await expect(loginHeading(page)).toBeVisible();

  const rejected = await submitLogin(page, user.nickname, "wrong-password");
  expect(rejected.status()).toBe(401);
  await expect(page.getByText("ニックネームまたはパスワードが違います。")).toBeVisible();
  await expect(loginHeading(page)).toBeVisible();
  expect(await findSessionCookie(context)).toBeUndefined();

  const accepted = await submitLogin(page, user.nickname, user.password);
  expect(accepted.status(), "登録済みユーザーのログインは新規作成にならない").toBe(200);
  expect(await accepted.json()).toEqual({ id: user.id, nickname: user.nickname, created: false });
  await expect(greeting(page, user.nickname)).toBeVisible();
  await expectSessionCookie(page);

  await expectNoAICalls(request);
});

test("無効なセッションでAPIが401を返すとログイン画面に切り替わり、再ログインで元の画面に戻る", async ({ page, context, request }) => {
  const user = await registerUser(page, "expired");
  await dismissMonthlyPrompt(page);

  // 署名の合わないCookieに差し替える（期限切れや SESSION_SECRET の変更後と同じ扱い）。
  await context.clearCookies();
  await context.addCookies([{ name: SESSION_COOKIE, value: "tampered.session", url: page.url(), httpOnly: true, sameSite: "Lax" }]);

  const rejected = page.waitForResponse(apiResponse("GET", "/api/calendar"));
  await page.getByRole("link", { name: "カレンダー", exact: true }).click();
  const response = await rejected;
  expect(response.status()).toBe(401);
  expect(await response.json()).toMatchObject({ status: 401, code: "UNAUTHENTICATED" });
  await expect(loginHeading(page)).toBeVisible();
  await expect(page).toHaveURL(/\/calendar$/);
  expect(await findSessionCookie(context), "バックエンドが中継越しに無効なCookieを削除する").toBeUndefined();

  const calendarLoaded = page.waitForResponse(apiResponse("GET", "/api/calendar"));
  const login = await submitLogin(page, user.nickname, user.password);
  expect(login.status()).toBe(200);
  expect((await calendarLoaded).status()).toBe(200);
  await expect(page.getByRole("heading", { level: 1, name: "カレンダー" })).toBeVisible();
  await expectSessionCookie(page);

  await expectNoAICalls(request);
});
