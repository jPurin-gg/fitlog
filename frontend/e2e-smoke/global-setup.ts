import type { FullConfig } from "@playwright/test";

const READY_TIMEOUT_MS = 120_000;

/**
 * Next.js の起動（webServer）後、バックエンドのDB接続・マイグレーション・種目seedの完了を待つ。
 * 未ログインの /api/auth/me が中継越しに Problem Details の401を返せば、
 * Next.js・中継先の設定・Goバックエンドがそろって応答できる。
 */
export default async function globalSetup(config: FullConfig) {
  const baseURL = config.projects[0]?.use.baseURL;
  if (!baseURL) throw new Error("playwright.smoke.config.ts に baseURL が設定されていません。");
  await waitForBackendThroughProxy(baseURL);
  await resetAIStubCalls();
}

async function waitForBackendThroughProxy(baseURL: string) {
  const url = new URL("/api/auth/me", baseURL);
  const deadline = Date.now() + READY_TIMEOUT_MS;
  let last = "応答なし";
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      const contentType = response.headers.get("content-type") ?? "";
      await response.text();
      if (response.status === 401 && contentType.includes("application/problem+json")) return;
      last = `HTTP ${response.status} ${contentType}`.trim();
    } catch (error) {
      last = error instanceof Error ? error.message : String(error);
    }
    await new Promise(resolve => setTimeout(resolve, 1_000));
  }
  throw new Error(`${url} が ${READY_TIMEOUT_MS / 1000} 秒以内にバックエンドの401を返しませんでした（最後の結果: ${last}）。backend サービスのログを確認してください。`);
}

async function resetAIStubCalls() {
  const stubURL = process.env.SMOKE_AI_STUB_URL;
  if (!stubURL) return;
  const response = await fetch(new URL("/__stub/calls", stubURL), { method: "DELETE" });
  if (!response.ok) throw new Error(`AIスタブの呼び出し記録を初期化できませんでした（HTTP ${response.status}）。`);
}
