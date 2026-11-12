import React from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AuthGate, LogoutButton, useAuth } from "./AuthGate";
import { AIToggle } from "./AISettings";
import { expectAccessible } from "@/test/a11y";

const origin = "http://localhost:3000";
const user = { id: 1, nickname: "テストユーザー" };
const server = setupServer(http.get(`${origin}/api/auth/me`, () => HttpResponse.json({}, { status: 401 })));
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => { vi.spyOn(console, "error").mockImplementation(() => {}); });
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function Content() {
  const { user } = useAuth();
  return <main><h1>{user.nickname}の記録</h1><AIToggle /><LogoutButton /></main>;
}
function show() { return render(<AuthGate><Content /></AuthGate>); }

it("セッション復元の完了まで保護画面を表示しない", async () => {
  let resolve!: (response: Response) => void;
  server.use(http.get(`${origin}/api/auth/me`, () => new Promise<Response>(done => { resolve = done; })));
  show();
  expect(screen.getByRole("status", { name: "セッションを確認中" })).toBeInTheDocument();
  expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  await waitFor(() => expect(resolve).toBeDefined());
  resolve(HttpResponse.json(user));
  expect(await screen.findByRole("heading", { name: "テストユーザーの記録" })).toBeInTheDocument();
  expect(screen.getByRole("switch", { name: "AIを使う" })).not.toBeChecked();
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});

it("未ログインではlabel付きフォームを表示し、入力後のログインで保護画面に入る", async () => {
  let input: unknown;
  server.use(http.post(`${origin}/api/auth/login`, async ({ request }) => {
    input = await request.json();
    return HttpResponse.json({ ...user, created: true });
  }));
  show();
  const nickname = await screen.findByRole("textbox", { name: "ニックネーム" });
  const password = screen.getByLabelText("パスワード");
  expect(password).toHaveAttribute("type", "password");
  expect(screen.getByRole("button", { name: "ログイン" })).toBeDisabled();
  await expectAccessible(screen.getByRole("main"));
  await userEvent.type(nickname, "テストユーザー");
  await userEvent.type(password, "test-password");
  await userEvent.click(screen.getByRole("button", { name: "ログイン" }));
  await screen.findByRole("heading", { name: "テストユーザーの記録" });
  expect(input).toEqual({ nickname: "テストユーザー", password: "test-password" });
});

it("ログイン失敗を表示して再試行できる", async () => {
  let calls = 0;
  server.use(http.post(`${origin}/api/auth/login`, () => ++calls === 1
    ? HttpResponse.json({ detail: "ログイン情報を確認してください。" }, { status: 401 })
    : HttpResponse.json(user)));
  show();
  await userEvent.type(await screen.findByLabelText("ニックネーム"), "テストユーザー");
  await userEvent.type(screen.getByLabelText("パスワード"), "test-password");
  await userEvent.click(screen.getByRole("button", { name: "ログイン" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("ログイン情報を確認してください。");
  expect(screen.getByLabelText("ニックネーム")).toHaveValue("テストユーザー");
  await userEvent.click(screen.getByRole("button", { name: "ログイン" }));
  await screen.findByRole("heading", { name: "テストユーザーの記録" });
  expect(calls).toBe(2);
});

it("復元APIが失敗しても読み込みを終えてログインできる", async () => {
  server.use(http.get(`${origin}/api/auth/me`, () => HttpResponse.json({}, { status: 503 })));
  show();
  await screen.findByRole("button", { name: "ログイン" });
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});

it.each([204, 500])("ログアウトAPIが%dでもローカルの認証状態を解除する", async status => {
  let calls = 0;
  server.use(
    http.get(`${origin}/api/auth/me`, () => HttpResponse.json(user)),
    http.delete(`${origin}/api/auth/session`, () => {
      calls++;
      return new HttpResponse(null, { status });
    }),
  );
  show();
  await userEvent.click(await screen.findByRole("button", { name: "ログアウト" }));
  await screen.findByRole("button", { name: "ログイン" });
  expect(calls).toBe(1);
  expect(screen.queryByRole("heading", { name: "テストユーザーの記録" })).not.toBeInTheDocument();
});

it("認証失効イベントでログイン画面へ戻る", async () => {
  server.use(http.get(`${origin}/api/auth/me`, () => HttpResponse.json(user)));
  show();
  await screen.findByRole("heading", { name: "テストユーザーの記録" });
  act(() => window.dispatchEvent(new Event("fitlog:unauthorized")));
  expect(screen.getByRole("button", { name: "ログイン" })).toBeInTheDocument();
});
