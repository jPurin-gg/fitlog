import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it } from "vitest";

import WorkoutHistoryPage from "./WorkoutHistoryPage";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

// ブラウザのタイムゾーンで 2026年8月{day}日 19:30 に始めた45分の記録。
function entry(id: number, day: number) {
  return {
    id,
    title: `記録${id}`,
    started_at: new Date(2026, 7, day, 19, 30).toISOString(),
    ended_at: new Date(2026, 7, day, 20, 15).toISOString(),
    duration_min: 45,
    total_sets: 12,
    total_volume: 3400,
  };
}

it("履歴を日付つきで一覧し、次のページを取得できる", async () => {
  const requests: string[] = [];
  server.use(http.get(`${apiOrigin}/api/workouts`, ({ request }) => {
    const url = new URL(request.url);
    requests.push(url.search);
    const offset = Number(url.searchParams.get("offset"));
    return HttpResponse.json({
      workouts: offset === 0
        ? Array.from({ length: 20 }, (_, index) => entry(40 - index, 24 - index))
        : [entry(20, 4)],
      total: 21,
      limit: 20,
      offset,
    });
  }));

  const user = userEvent.setup();
  render(<WorkoutHistoryPage />);

  const list = await screen.findByRole("list", { name: "ワークアウト履歴の一覧" });
  expect(within(list).getAllByRole("listitem")).toHaveLength(20);
  const newest = within(list).getByRole("link", { name: /記録40/ });
  expect(newest).toHaveAttribute("href", "/workouts/40");
  expect(within(newest).getByText(/2026年8月24日\(月\)\s*19:30/)).toBeInTheDocument();
  expect(within(newest).getByText("45分 ・ 12セット ・ 3,400kg")).toBeInTheDocument();
  expect(screen.getByText("1〜20件 / 全21件")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "前へ" })).toBeDisabled();

  await user.click(screen.getByRole("button", { name: "次へ" }));

  expect(await screen.findByRole("link", { name: /記録20/ })).toHaveAttribute("href", "/workouts/20");
  expect(screen.getByText("21〜21件 / 全21件")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "次へ" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "前へ" })).toBeEnabled();
  expect(requests).toEqual(["?limit=20&offset=0", "?limit=20&offset=20"]);
});

it("履歴がないときは記録開始へ案内する", async () => {
  server.use(http.get(`${apiOrigin}/api/workouts`, () => HttpResponse.json({ workouts: [], total: 0, limit: 20, offset: 0 })));

  render(<WorkoutHistoryPage />);

  expect(await screen.findByText("まだ完了したワークアウトはありません。")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "記録を始める" })).toHaveAttribute("href", "/workout");
  expect(screen.queryByRole("button", { name: "次へ" })).not.toBeInTheDocument();
});
