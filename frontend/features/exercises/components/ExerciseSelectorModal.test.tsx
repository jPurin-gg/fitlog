import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { ExerciseSelectorModal } from "./ExerciseSelectorModal";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

function useListHandlers() {
  server.use(
    http.get(`${apiOrigin}/api/exercises/favorites`, () => HttpResponse.json([])),
    http.get(`${apiOrigin}/api/exercises/recent`, () => HttpResponse.json([])),
    http.get(`${apiOrigin}/api/exercises`, () => HttpResponse.json([{ id: "squat", name: "スクワット", category: "strength" }])),
  );
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

describe("ExerciseSelectorModal の失敗表示", () => {
  it("お気に入りを更新できないとき、alertを使わず画面内に日本語で表示する", async () => {
    useListHandlers();
    server.use(
      http.put(`${apiOrigin}/api/exercises/squat/favorite`, () => new HttpResponse(
        "<html><body><h1>502 Bad Gateway</h1></body></html>",
        { status: 502, headers: { "Content-Type": "text/html" } },
      )),
    );
    const browserAlert = vi.fn();
    vi.stubGlobal("alert", browserAlert);
    const user = userEvent.setup();
    render(<ExerciseSelectorModal onClose={() => undefined} onSelect={() => undefined} />);

    await user.click(screen.getByRole("button", { name: "検索" }));
    await user.click(await screen.findByRole("button", { name: "お気に入りに追加" }));

    const message = await screen.findByRole("alert");
    expect(message).toHaveTextContent("サーバーに接続できませんでした。少し待ってからもう一度お試しください。");
    expect(message).not.toHaveTextContent(/Bad Gateway|</);
    expect(screen.getByRole("button", { name: "お気に入りに追加" })).toBeInTheDocument();
    expect(browserAlert).not.toHaveBeenCalled();
  });

  it("種目を追加できないとき、原因に応じた文言を画面内に表示する", async () => {
    useListHandlers();
    let calls = 0;
    server.use(
      http.post(`${apiOrigin}/api/exercises`, () => {
        calls += 1;
        return calls === 1
          ? HttpResponse.json(
            { status: 400, detail: "種目名を確認してください。" },
            { status: 400, headers: { "Content-Type": "application/problem+json" } },
          )
          : HttpResponse.error();
      }),
    );
    const browserAlert = vi.fn();
    vi.stubGlobal("alert", browserAlert);
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<ExerciseSelectorModal onClose={() => undefined} onSelect={onSelect} />);

    await user.click(screen.getByRole("button", { name: "見つからない種目を新しく追加" }));
    await user.type(screen.getByPlaceholderText("例: ダンベルプレス"), "マイ種目");
    await user.click(screen.getByRole("button", { name: "追加して選択" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("種目名を確認してください。");

    await user.click(screen.getByRole("button", { name: "追加して選択" }));
    expect(await screen.findByText("種目を追加できませんでした。通信環境を確認して、もう一度お試しください。")).toBeInTheDocument();
    expect(calls).toBe(2);
    expect(onSelect).not.toHaveBeenCalled();
    expect(browserAlert).not.toHaveBeenCalled();
  });
});
