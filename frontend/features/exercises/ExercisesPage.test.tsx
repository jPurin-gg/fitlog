import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";

import ExercisesPage from "./ExercisesPage";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

const filterOptions = {
  equipment: ["自重", "EZバー", "チューブ・バンド", "バランスボール", "フォームローラー"],
  muscle_categories: [
    { name: "背中", muscles: ["広背筋", "僧帽筋"] },
    { name: "脚", muscles: ["大腿四頭筋", "内転筋", "外転筋"] },
  ],
};

function summary(id: string, name: string) {
  return { id, name, level: "初級", equipment: "自重", category: "筋力トレーニング", primaryMuscles: ["広背筋"] };
}

function page(items: ReturnType<typeof summary>[], total = items.length, offset = 0) {
  return { items, total, limit: 50, offset };
}

// 検索以外のAPIは共通の応答にし、検索の応答だけをテストごとに決める。
function mockSearchAPI(search: (params: URLSearchParams) => Response | Promise<Response>) {
  server.use(
    http.get(`${apiOrigin}/api/exercises/filters`, () => HttpResponse.json(filterOptions)),
    http.get(`${apiOrigin}/api/exercises`, ({ request }) => search(new URL(request.url).searchParams)),
  );
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

it("部位・器具の選択肢をAPIから表示し、選んだ値をそのまま検索条件にする", async () => {
  const queries: string[] = [];
  mockSearchAPI(params => {
    queries.push(`${params.get("muscle") ?? ""}|${params.get("equipment") ?? ""}`);
    return HttpResponse.json(page([summary("pull_up", "懸垂")]));
  });
  const user = userEvent.setup();
  render(<ExercisesPage />);

  await screen.findByText("懸垂");
  for (const name of ["内転筋", "外転筋", "EZバー", "フォームローラー", "チューブ・バンド", "バランスボール"]) {
    expect(await screen.findByRole("button", { name })).toBeInTheDocument();
  }
  expect(screen.queryByRole("button", { name: "バンド" })).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "内転筋" }));
  await waitFor(() => expect(queries.at(-1)).toBe("内転筋|"));
  await user.click(screen.getByRole("button", { name: "チューブ・バンド" }));
  await user.click(screen.getByRole("button", { name: "バランスボール" }));
  await waitFor(() => expect(queries.at(-1)).toBe("内転筋|チューブ・バンド,バランスボール"));
});

it("取得に失敗したときは「見つかりませんでした」ではなくエラーと再試行を表示する", async () => {
  let calls = 0;
  mockSearchAPI(() => {
    calls++;
    return calls === 1 ? HttpResponse.error() : HttpResponse.json(page([summary("pull_up", "懸垂")]));
  });
  const user = userEvent.setup();
  render(<ExercisesPage />);

  expect(await screen.findByText("種目を読み込めませんでした")).toBeInTheDocument();
  expect(screen.getByText(/種目を取得できませんでした/)).toBeInTheDocument();
  expect(screen.queryByText("見つかりませんでした")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "再試行" }));
  expect(await screen.findByText("懸垂")).toBeInTheDocument();
  expect(screen.queryByText("種目を読み込めませんでした")).not.toBeInTheDocument();
  expect(calls).toBe(2);
});

it("一覧を1ページずつ描画し、さらに表示で続きを取得する", async () => {
  const all = Array.from({ length: 60 }, (_, index) => {
    const number = String(index + 1).padStart(2, "0");
    return summary(`exercise_${number}`, `種目${number}`);
  });
  const offsets: number[] = [];
  mockSearchAPI(params => {
    const offset = Number(params.get("offset") ?? "0");
    offsets.push(offset);
    return HttpResponse.json(page(all.slice(offset, offset + 50), all.length, offset));
  });
  const user = userEvent.setup();
  render(<ExercisesPage />);

  expect(await screen.findByText("種目50")).toBeInTheDocument();
  expect(screen.queryByText("種目51")).not.toBeInTheDocument();
  expect(screen.getAllByRole("button", { name: "詳細を見る" })).toHaveLength(50);
  expect(screen.getByText("（50 件を表示中）")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "さらに表示" }));
  expect(await screen.findByText("種目60")).toBeInTheDocument();
  expect(screen.getAllByRole("button", { name: "詳細を見る" })).toHaveLength(60);
  expect(screen.queryByRole("button", { name: "さらに表示" })).not.toBeInTheDocument();
  expect(offsets).toEqual([0, 50]);
});

it("手順文は一覧に含めず、詳細を開いたときに取得する", async () => {
  let detailCalls = 0;
  mockSearchAPI(() => HttpResponse.json(page([summary("pull_up", "懸垂")])));
  server.use(http.get(`${apiOrigin}/api/exercises/pull_up`, () => {
    detailCalls++;
    if (detailCalls === 1) return HttpResponse.error();
    return HttpResponse.json({
      ...summary("pull_up", "懸垂"),
      force: "引く(プル)",
      mechanic: "コンパウンド(多関節)",
      instructions: ["バーを肩幅より広く握ります。"],
      secondaryMuscles: ["上腕二頭筋"],
      images: [],
    });
  }));
  const user = userEvent.setup();
  render(<ExercisesPage />);

  await user.click(await screen.findByRole("button", { name: "詳細を見る" }));
  expect(await screen.findByText(/種目の詳細を取得できませんでした/)).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "手順を再取得" }));
  expect(await screen.findByText("バーを肩幅より広く握ります。")).toBeInTheDocument();
  expect(screen.getByText("上腕二頭筋")).toBeInTheDocument();
  expect(detailCalls).toBe(2);
});
