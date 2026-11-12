import React from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { expectAccessible } from "@/test/a11y";
import ExercisesPage from "./ExercisesPage";

const endpoint = "http://localhost:3000/api/exercises";
const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it("種目取得の失敗を検索結果なしと区別し、再試行で一覧を表示する", async () => {
  let calls = 0;
  server.use(http.get(endpoint, () => {
    calls++;
    return calls === 1
      ? HttpResponse.json({ detail: "種目を取得できませんでした。" }, { status: 500 })
      : HttpResponse.json([{ id: "bench", name: "ベンチプレス" }]);
  }));
  render(<ExercisesPage />);
  const error = await screen.findByRole("alert");
  expect(error).toHaveTextContent("種目を取得できませんでした。");
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
  expect(screen.queryByText("見つかりませんでした")).not.toBeInTheDocument();
  await userEvent.click(within(error).getByRole("button", { name: "再試行" }));
  expect(await screen.findByRole("heading", { name: "ベンチプレス" })).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(calls).toBe(2);
});

it("検索条件を変えた後に以前の検索結果を表示しない", async () => {
  let resolvePrevious!: (response: Response) => void;
  const previous = new Promise<Response>(resolve => { resolvePrevious = resolve; });
  const names: (string | null)[] = [];
  server.use(http.get(endpoint, ({ request }) => {
    const name = new URL(request.url).searchParams.get("name");
    names.push(name);
    return name ? HttpResponse.json([{ id: "squat", name: "スクワット" }]) : previous;
  }));
  render(<ExercisesPage />);
  await waitFor(() => expect(names).toHaveLength(1));
  await userEvent.type(screen.getByRole("textbox", { name: "種目名や器具で検索" }), "スクワット");
  expect(await screen.findByRole("heading", { name: "スクワット" })).toBeInTheDocument();
  await act(async () => { resolvePrevious(HttpResponse.json([{ id: "bench", name: "ベンチプレス" }])); });
  expect(screen.queryByRole("heading", { name: "ベンチプレス" })).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "スクワット" })).toBeInTheDocument();
  expect(names).toEqual([null, "スクワット"]);
});

it("部位・難易度・器具の選択条件をAPIに送る", async () => {
  let query = new URLSearchParams();
  server.use(http.get(endpoint, ({ request }) => {
    query = new URL(request.url).searchParams;
    return HttpResponse.json([]);
  }));
  render(<ExercisesPage />);
  await userEvent.click(screen.getByRole("button", { name: "大胸筋" }));
  await userEvent.click(screen.getByRole("button", { name: "初級" }));
  await userEvent.click(screen.getByRole("button", { name: "ダンベル" }));
  await waitFor(() => expect(Object.fromEntries(query)).toEqual({ muscle: "大胸筋", level: "初級", equipment: "ダンベル" }));
});

it("種目詳細と追加フォームを名前付きdialogで開き、Escapeで元のボタンに戻る", async () => {
  server.use(http.get(endpoint, () => HttpResponse.json([{ id: "bench", name: "ベンチプレス", instructions: ["胸を張る"] }])));
  render(<ExercisesPage />);
  const details = await screen.findByRole("button", { name: "詳細を見る" });
  await userEvent.click(details);
  const dialog = screen.getByRole("dialog", { name: "ベンチプレスの詳細" });
  expect(within(dialog).getByText("胸を張る")).toBeInTheDocument();
  await expectAccessible(dialog);
  await userEvent.keyboard("{Escape}");
  expect(details).toHaveFocus();
  const add = screen.getByRole("button", { name: "種目を追加" });
  await userEvent.click(add);
  await expectAccessible(screen.getByRole("dialog", { name: "種目を追加" }));
  await userEvent.keyboard("{Escape}");
  expect(add).toHaveFocus();
});
