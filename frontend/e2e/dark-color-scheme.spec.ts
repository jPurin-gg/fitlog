import { expect, test, type Page } from "@playwright/test";

// OS がライト配色の端末でも、画面はダーク固定で描かれ、下部に白い領域が出ないことを確かめる（#92）。
test.use({ colorScheme: "light" });

const DARK_BACKGROUND = "rgb(10, 10, 10)";

async function mockAPI(page: Page, { signedIn }: { signedIn: boolean }) {
  await page.route("**/api/**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const respond = (status: number, body: unknown) => route.fulfill({
      status,
      contentType: status >= 400 ? "application/problem+json" : "application/json",
      body: JSON.stringify(body),
    });
    if (request.method() !== "GET") return respond(404, { detail: "Unexpected test request" });
    if (path === "/api/auth/me") {
      return signedIn ? respond(200, { id: 1, nickname: "E2E user" }) : respond(401, { detail: "ログインしてください。" });
    }
    if (path === "/api/dashboard") return respond(200, { recent_workouts: [] });
    if (path === "/api/preferences") return respond(200, {});
    if (/^\/api\/monthly-plans\/\d{4}-\d{2}$/.test(path)) return respond(404, { detail: "月間プランはありません。" });
    if (/^\/api\/workout-plans\//.test(path)) return respond(404, { detail: "予定はありません。" });
    if (path.startsWith("/api/exercises")) return respond(200, []);
    if (path.startsWith("/api/workouts")) return respond(200, []);
    return respond(404, { detail: "Unexpected test request" });
  });
}

async function expectDarkCanvas(page: Page) {
  const colors = await page.evaluate(() => ({
    scheme: getComputedStyle(document.documentElement).colorScheme,
    body: getComputedStyle(document.body).backgroundColor,
    themeColor: document.querySelector('meta[name="theme-color"]')?.getAttribute("content"),
  }));
  expect(colors).toEqual({ scheme: "dark", body: DARK_BACKGROUND, themeColor: "#0a0a0a" });
}

async function scrollOverflow(page: Page) {
  return page.evaluate(() => document.scrollingElement!.scrollHeight - window.innerHeight);
}

test("ログイン画面はライト配色でもダークで描かれ、縦スクロールが出ない", async ({ page }) => {
  await mockAPI(page, { signedIn: false });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Fitlog に入る" })).toBeVisible();
  await expectDarkCanvas(page);
  expect(await scrollOverflow(page)).toBeLessThanOrEqual(0);
});

for (const path of ["/", "/workout", "/calendar", "/exercises"]) {
  test(`${path} はライト配色でも最下部まで暗い背景で描かれる`, async ({ page }) => {
    await mockAPI(page, { signedIn: true });
    await page.goto(path);
    await expect(page.getByRole("navigation")).toBeVisible();
    await expectDarkCanvas(page);

    await page.evaluate(() => window.scrollTo(0, document.scrollingElement!.scrollHeight));
    // 最下部の点にある要素をたどり、最初に不透明な背景を持つものが暗色であること。
    const bottomBackground = await page.evaluate(() => {
      const stack = document.elementsFromPoint(4, window.innerHeight - 2);
      for (const element of stack) {
        const color = getComputedStyle(element).backgroundColor;
        if (color !== "rgba(0, 0, 0, 0)" && color !== "transparent") return color;
      }
      return getComputedStyle(document.body).backgroundColor;
    });
    expect(bottomBackground).toBe(DARK_BACKGROUND);
  });
}
