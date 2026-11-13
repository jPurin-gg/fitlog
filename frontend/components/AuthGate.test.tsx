import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { AuthGate, useAuth } from "./AuthGate";
import { apiTimeouts } from "@/lib/api";

const apiOrigin = "http://localhost:3000";
const server = setupServer();
const authUser = { id: 1, nickname: "テストユーザー" };
const defaultStandardTimeoutMs = apiTimeouts.standardMs;

function Content() {
  const { user } = useAuth();
  return <h1>{user.nickname}の記録</h1>;
}

function renderGate() {
  return render(<AuthGate><Content /></AuthGate>);
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  server.resetHandlers();
  apiTimeouts.standardMs = defaultStandardTimeoutMs;
});
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

describe("AuthGate のセッション確認", () => {
  it("応答しないセッション確認を待ち時間で打ち切り、再確認できる", async () => {
    // 応答しないサーバーを再現するため、最初のリクエストには最後まで応答しない。
    const hung = new Promise<Response>(() => undefined);
    let calls = 0;
    server.use(
      http.get(`${apiOrigin}/api/auth/me`, () => {
        calls += 1;
        return calls === 1 ? hung : HttpResponse.json(authUser);
      }),
    );
    apiTimeouts.standardMs = 100;
    const user = userEvent.setup();
    renderGate();

    const notice = await screen.findByRole("alert");
    expect(notice).toHaveTextContent("ログイン状態を確認できませんでした");
    expect(notice).toHaveTextContent("サーバーからの応答がありませんでした。通信環境を確認して、もう一度お試しください。");
    expect(screen.getByRole("button", { name: "ログイン" })).toBeInTheDocument();

    apiTimeouts.standardMs = defaultStandardTimeoutMs;
    await user.click(screen.getByRole("button", { name: "ログイン状態を再確認" }));

    expect(await screen.findByRole("heading", { name: "テストユーザーの記録" })).toBeInTheDocument();
    expect(calls).toBe(2);
  });

  it("中継がHTMLのエラーを返しても生の本文を出さず、そのままログインできる", async () => {
    server.use(
      http.get(`${apiOrigin}/api/auth/me`, () => new HttpResponse(
        "<html><body><h1>502 Bad Gateway</h1></body></html>",
        { status: 502, headers: { "Content-Type": "text/html" } },
      )),
      http.post(`${apiOrigin}/api/auth/login`, () => HttpResponse.json({ ...authUser, created: false })),
    );
    const user = userEvent.setup();
    renderGate();

    const notice = await screen.findByRole("alert");
    expect(notice).toHaveTextContent("サーバーに接続できませんでした。少し待ってからもう一度お試しください。");
    expect(notice).not.toHaveTextContent(/Bad Gateway|</);

    await user.type(screen.getByPlaceholderText("例: 筋トレマン"), "テストユーザー");
    await user.type(screen.getByPlaceholderText("パスワード"), "test-password");
    await user.click(screen.getByRole("button", { name: "ログイン" }));

    expect(await screen.findByRole("heading", { name: "テストユーザーの記録" })).toBeInTheDocument();
  });

  it("未ログインでは確認失敗の案内を出さない", async () => {
    server.use(
      http.get(`${apiOrigin}/api/auth/me`, () => HttpResponse.json(
        { status: 401, detail: "ログインが必要です。" },
        { status: 401, headers: { "Content-Type": "application/problem+json" } },
      )),
    );
    renderGate();

    expect(await screen.findByRole("button", { name: "ログイン" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
