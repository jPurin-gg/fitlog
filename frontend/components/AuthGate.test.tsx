import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { apiFetch } from "@/lib/api";
import { AuthGate, LogoutButton, useAuth } from "./AuthGate";

const origin = "http://localhost:3000";
const account = { id: 1, nickname: "Mitsuki" };
const server = setupServer(
  http.get(`${origin}/api/auth/me`, () => HttpResponse.json({ detail: "ログインが必要です。" }, { status: 401 })),
);
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => { vi.spyOn(console, "error").mockImplementation(() => {}); });
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function Content() {
  const { user, isNewAccount } = useAuth();
  const saveSet = () => {
    void apiFetch("/api/workouts/42/sets", { method: "POST", body: "{}" }).catch(() => {});
  };
  return (
    <main>
      <h1>{user.nickname}の記録</h1>
      <p>{isNewAccount ? "作成したばかりのアカウント" : "既存のアカウント"}</p>
      <button onClick={saveSet}>セットを保存</button>
      <LogoutButton />
    </main>
  );
}
function show() { return render(<AuthGate><Content /></AuthGate>); }

async function fillCredentials(nickname: string, password: string) {
  await userEvent.type(await screen.findByLabelText("ニックネーム"), nickname);
  await userEvent.type(screen.getByLabelText("パスワード"), password);
}

it("未登録のニックネームでログインしてもアカウントを作成しない", async () => {
  let registrations = 0;
  server.use(
    http.post(`${origin}/api/auth/login`, () => HttpResponse.json({ detail: "ニックネームまたはパスワードが違います。" }, { status: 401 })),
    http.post(`${origin}/api/auth/register`, () => {
      registrations++;
      return HttpResponse.json({ ...account, created: true }, { status: 201 });
    }),
  );
  show();
  await fillCredentials("Mitsuko", "password");
  expect(screen.getByRole("radio", { name: "ログイン" })).toBeChecked();
  expect(screen.queryByLabelText("パスワード（確認）")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "ログイン" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("ニックネームまたはパスワードが違います。");
  expect(registrations).toBe(0);
  expect(screen.queryByRole("heading", { name: "Mitsukiの記録" })).not.toBeInTheDocument();
  // ログイン失敗の401は、セッション切れとしては案内しない。
  expect(screen.queryByText(/セッションの有効期限が切れました/)).not.toBeInTheDocument();
});

it("登録済みのアカウントにはログインで入り、作成の通知を出さない", async () => {
  let input: unknown;
  server.use(http.post(`${origin}/api/auth/login`, async ({ request }) => {
    input = await request.json();
    return HttpResponse.json({ ...account, created: false });
  }));
  show();
  await fillCredentials("Mitsuki", "password");
  await userEvent.click(screen.getByRole("button", { name: "ログイン" }));
  expect(await screen.findByRole("heading", { name: "Mitsukiの記録" })).toBeInTheDocument();
  expect(screen.getByText("既存のアカウント")).toBeInTheDocument();
  expect(input).toEqual({ nickname: "Mitsuki", password: "password" });
});

it("新規登録はパスワード確認が一致するまで送信できず、作成直後に必ず通知する", async () => {
  const inputs: unknown[] = [];
  server.use(http.post(`${origin}/api/auth/register`, async ({ request }) => {
    inputs.push(await request.json());
    return HttpResponse.json({ ...account, created: true }, { status: 201 });
  }));
  show();
  await userEvent.click(await screen.findByRole("radio", { name: "新規登録" }));
  await fillCredentials("Mitsuki", "password");
  const submit = screen.getByRole("button", { name: "アカウントを作成" });
  const confirmation = screen.getByLabelText("パスワード（確認）");
  expect(submit).toBeDisabled();

  await userEvent.type(confirmation, "passwrd{Enter}");
  expect(screen.getByText("パスワードが一致しません。")).toBeInTheDocument();
  expect(confirmation).toHaveAttribute("aria-invalid", "true");
  expect(submit).toBeDisabled();
  expect(inputs).toEqual([]);

  await userEvent.clear(confirmation);
  await userEvent.type(confirmation, "password");
  expect(screen.queryByText("パスワードが一致しません。")).not.toBeInTheDocument();
  expect(submit).toBeEnabled();
  await userEvent.click(submit);

  expect(await screen.findByRole("status")).toHaveTextContent("新しいアカウント「Mitsuki」を作成しました");
  expect(inputs).toEqual([{ nickname: "Mitsuki", password: "password" }]);
  expect(screen.queryByRole("heading", { name: "Mitsukiの記録" })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "はじめる" }));
  expect(await screen.findByRole("heading", { name: "Mitsukiの記録" })).toBeInTheDocument();
  expect(screen.getByText("作成したばかりのアカウント")).toBeInTheDocument();
});

it("作成の通知からログイン画面に戻れる", async () => {
  let logouts = 0;
  server.use(
    http.post(`${origin}/api/auth/register`, () => HttpResponse.json({ ...account, created: true }, { status: 201 })),
    http.delete(`${origin}/api/auth/session`, () => {
      logouts++;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  show();
  await userEvent.click(await screen.findByRole("radio", { name: "新規登録" }));
  await fillCredentials("Mitsuki", "password");
  await userEvent.type(screen.getByLabelText("パスワード（確認）"), "password");
  await userEvent.click(screen.getByRole("button", { name: "アカウントを作成" }));
  await userEvent.click(await screen.findByRole("button", { name: "ログイン画面に戻る" }));
  expect(await screen.findByRole("button", { name: "ログイン" })).toBeInTheDocument();
  expect(logouts).toBe(1);
  expect(screen.queryByRole("heading", { name: "Mitsukiの記録" })).not.toBeInTheDocument();
});

it("ニックネーム欄は自動大文字化と自動修正を抑止する", async () => {
  show();
  const nickname = await screen.findByLabelText("ニックネーム");
  expect(nickname).toHaveAttribute("autocapitalize", "none");
  expect(nickname).toHaveAttribute("autocorrect", "off");
  expect(nickname).toHaveAttribute("spellcheck", "false");
});

it.each(["500", "通信失敗"])("セッション確認が%sのときはログインフォームではなく再試行を表示する", async kind => {
  let calls = 0;
  server.use(http.get(`${origin}/api/auth/me`, () => {
    if (++calls > 1) return HttpResponse.json(account);
    return kind === "500" ? HttpResponse.json({ detail: "処理中にエラーが発生しました。" }, { status: 500 }) : HttpResponse.error();
  }));
  show();
  expect(await screen.findByRole("heading", { name: "サーバーに接続できません" })).toBeInTheDocument();
  expect(screen.queryByLabelText("ニックネーム")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "ログイン" })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "再試行" }));
  expect(await screen.findByRole("heading", { name: "Mitsukiの記録" })).toBeInTheDocument();
  expect(calls).toBe(2);
});

it("記録中に401が返ると、ログイン画面に期限切れの説明を出す", async () => {
  server.use(
    http.get(`${origin}/api/auth/me`, () => HttpResponse.json(account)),
    http.post(`${origin}/api/workouts/42/sets`, () => HttpResponse.json({ detail: "セッションが無効または期限切れです。" }, { status: 401 })),
    http.post(`${origin}/api/auth/login`, () => HttpResponse.json({ ...account, created: false })),
  );
  show();
  await userEvent.click(await screen.findByRole("button", { name: "セットを保存" }));
  expect(await screen.findByRole("status")).toHaveTextContent("セッションの有効期限が切れました。もう一度ログインしてください。");
  expect(screen.queryByRole("heading", { name: "Mitsukiの記録" })).not.toBeInTheDocument();
  // 期限切れの後は新規登録ではなくログインが選ばれている。
  expect(screen.getByRole("radio", { name: "ログイン" })).toBeChecked();

  await fillCredentials("Mitsuki", "password");
  await userEvent.click(screen.getByRole("button", { name: "ログイン" }));
  expect(await screen.findByRole("heading", { name: "Mitsukiの記録" })).toBeInTheDocument();
  expect(screen.queryByText(/セッションの有効期限が切れました/)).not.toBeInTheDocument();
});

it("自分でログアウトしたときは、APIが401でも期限切れと表示しない", async () => {
  server.use(
    http.get(`${origin}/api/auth/me`, () => HttpResponse.json(account)),
    http.delete(`${origin}/api/auth/session`, () => HttpResponse.json({ detail: "セッションが無効または期限切れです。" }, { status: 401 })),
  );
  show();
  await userEvent.click(await screen.findByRole("button", { name: "ログアウト" }));
  expect(await screen.findByRole("button", { name: "ログイン" })).toBeInTheDocument();
  await waitFor(() => expect(screen.queryByText(/セッションの有効期限が切れました/)).not.toBeInTheDocument());
});
