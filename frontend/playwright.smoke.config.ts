import { defineConfig, devices } from "@playwright/test";

// 実APIを通すスモークE2E。APIはモックせず、Next.js の /api 中継越しに Go バックエンドと一時DBへ到達する。
// 通常は compose.test.yml の e2e-smoke サービスから実行する（ルートの README を参照）。
// SMOKE_BASE_URL を指定すると、Next.js を起動せずに起動済みの環境へ向けて実行する。
const port = process.env.SMOKE_PORT || "3000";
const externalBaseURL = process.env.SMOKE_BASE_URL;
const baseURL = externalBaseURL || `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./e2e-smoke",
  outputDir: "./smoke-results/test-results",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  timeout: 90_000,
  expect: { timeout: 15_000 },
  globalSetup: "./e2e-smoke/global-setup.ts",
  reporter: [["list"], ["html", { outputFolder: "./smoke-results/report", open: "never" }]],
  use: {
    baseURL,
    locale: "ja-JP",
    // 開始APIはサーバーの APP_TIMEZONE で「当日」を判定し、画面はブラウザのローカル日付を送る。両者をそろえる。
    timezoneId: "Asia/Tokyo",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: externalBaseURL ? undefined : {
    // BACKEND_INTERNAL_URL を指定して `next build` 済みの本番サーバーを起動する（rewrites はビルド時に確定する）。
    command: `npm run start -- --hostname 127.0.0.1 --port ${port}`,
    url: baseURL,
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
