import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";

import { ExerciseSelectorModal } from "./ExerciseSelectorModal";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

const filterOptions = {
  equipment: ["自重", "ダンベル", "チューブ・バンド", "バランスボール"],
  muscle_categories: [
    { name: "胸", muscles: ["大胸筋"] },
    { name: "背中", muscles: ["広背筋", "背中中部", "下背部", "僧帽筋"] },
    { name: "腕", muscles: ["上腕二頭筋", "上腕三頭筋", "前腕"] },
  ],
};

interface Summary {
  id: string;
  name: string;
  level: string | null;
  equipment: string | null;
  category: string | null;
  primaryMuscles: string[];
}

function summary(id: string, name: string, primaryMuscle = "広背筋"): Summary {
  return { id, name, level: "初級", equipment: "自重", category: "筋力トレーニング", primaryMuscles: [primaryMuscle] };
}

function page(items: Summary[], total = items.length, offset = 0) {
  return { items, total, limit: 50, offset };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(next => {
    resolve = next;
  });
  return { promise, resolve };
}

// 検索以外のAPIは共通の応答にし、検索の応答だけをテストごとに決める。
function mockSearchAPI(search: (params: URLSearchParams) => Response | Promise<Response>) {
  server.use(
    http.get(`${apiOrigin}/api/exercises/filters`, () => HttpResponse.json(filterOptions)),
    http.get(`${apiOrigin}/api/exercises/favorites`, () => HttpResponse.json([])),
    http.get(`${apiOrigin}/api/exercises/recent`, () => HttpResponse.json([])),
    http.get(`${apiOrigin}/api/exercises`, ({ request }) => search(new URL(request.url).searchParams)),
  );
}

async function openSearchTab(user: ReturnType<typeof userEvent.setup>) {
  render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={vi.fn()} />);
  await user.click(screen.getByRole("button", { name: "検索" }));
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

it("お気に入りタブでは部位・器具の絞り込みを表示せず、検索タブを開くまで一覧を取得しない", async () => {
  let searches = 0;
  mockSearchAPI(() => {
    searches++;
    return HttpResponse.json(page([summary("pull_up", "懸垂")]));
  });
  const user = userEvent.setup();
  render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={vi.fn()} />);

  expect(await screen.findByText(/まだお気に入りがありません/)).toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "大まかな部位" })).not.toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "器具" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "最近" }));
  expect(screen.queryByRole("combobox", { name: "細かい筋肉" })).not.toBeInTheDocument();
  expect(searches).toBe(0);

  await user.click(screen.getByRole("button", { name: "検索" }));
  expect(await screen.findByText("懸垂")).toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "大まかな部位" })).toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "器具" })).toBeInTheDocument();
  expect(searches).toBe(1);
});

it("大分類「背中」は分類内の全部位で検索し、背中中部・下背部・僧帽筋の種目も表示する", async () => {
  const catalog = [
    summary("lat_pulldown", "ラットプルダウン", "広背筋"),
    summary("seated_row", "シーテッドロー", "背中中部"),
    summary("back_extension", "バックエクステンション", "下背部"),
    summary("shrug", "シュラッグ", "僧帽筋"),
    summary("bench", "ベンチプレス", "大胸筋"),
  ];
  const muscleParams: (string | null)[] = [];
  mockSearchAPI(params => {
    const muscle = params.get("muscle");
    muscleParams.push(muscle);
    const wanted = muscle ? muscle.split(",") : null;
    return HttpResponse.json(page(catalog.filter(item => !wanted || wanted.includes(item.primaryMuscles[0]))));
  });
  const user = userEvent.setup();
  await openSearchTab(user);
  expect(await screen.findByText("ベンチプレス")).toBeInTheDocument();
  await screen.findByRole("option", { name: "背中" });

  await user.selectOptions(screen.getByRole("combobox", { name: "大まかな部位" }), "背中");
  await waitFor(() => expect(screen.queryByText("ベンチプレス")).not.toBeInTheDocument());
  for (const name of ["ラットプルダウン", "シーテッドロー", "バックエクステンション", "シュラッグ"]) {
    expect(await screen.findByText(name)).toBeInTheDocument();
  }
  expect(muscleParams.at(-1)).toBe("広背筋,背中中部,下背部,僧帽筋");

  await user.selectOptions(screen.getByRole("combobox", { name: "細かい筋肉" }), "僧帽筋");
  await waitFor(() => expect(muscleParams.at(-1)).toBe("僧帽筋"));
  await waitFor(() => expect(screen.queryByText("ラットプルダウン")).not.toBeInTheDocument());
  expect(await screen.findByText("シュラッグ")).toBeInTheDocument();
});

it("器具の選択肢はAPIが返す表記をそのまま表示して送信する", async () => {
  const equipmentParams: (string | null)[] = [];
  mockSearchAPI(params => {
    equipmentParams.push(params.get("equipment"));
    return HttpResponse.json(page([summary("band_row", "バンドロー")]));
  });
  const user = userEvent.setup();
  await openSearchTab(user);
  await screen.findByText("バンドロー");
  await screen.findByRole("option", { name: "チューブ・バンド" });
  expect(screen.getByRole("option", { name: "バランスボール" })).toBeInTheDocument();
  expect(screen.queryByRole("option", { name: "バンド" })).not.toBeInTheDocument();

  await user.selectOptions(screen.getByRole("combobox", { name: "器具" }), "チューブ・バンド");
  await waitFor(() => expect(equipmentParams.at(-1)).toBe("チューブ・バンド"));
});

it("一覧は1ページずつ取得し、さらに表示で51件目以降に到達できる", async () => {
  const all = Array.from({ length: 60 }, (_, index) => {
    const number = String(index + 1).padStart(2, "0");
    return summary(`exercise_${number}`, `種目${number}`);
  });
  const requests: { limit: string | null; offset: number }[] = [];
  mockSearchAPI(params => {
    const offset = Number(params.get("offset") ?? "0");
    requests.push({ limit: params.get("limit"), offset });
    return HttpResponse.json(page(all.slice(offset, offset + 50), all.length, offset));
  });
  const user = userEvent.setup();
  await openSearchTab(user);

  expect(await screen.findByText("種目50")).toBeInTheDocument();
  expect(screen.queryByText("種目51")).not.toBeInTheDocument();
  expect(screen.getByText("60件中 50件を表示")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "さらに表示" }));
  expect(await screen.findByText("種目60")).toBeInTheDocument();
  expect(screen.getByText("種目01")).toBeInTheDocument();
  expect(screen.getByText("60件中 60件を表示")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "さらに表示" })).not.toBeInTheDocument();
  expect(requests).toEqual([{ limit: "50", offset: 0 }, { limit: "50", offset: 50 }]);
});

it("取得に失敗したときはエラーメッセージと再試行を表示し、再試行で一覧を表示する", async () => {
  let calls = 0;
  mockSearchAPI(() => {
    calls++;
    if (calls === 1) {
      return HttpResponse.json(
        { status: 500, detail: "処理中にエラーが発生しました。" },
        { status: 500, headers: { "Content-Type": "application/problem+json" } },
      );
    }
    return HttpResponse.json(page([summary("pull_up", "懸垂")]));
  });
  const user = userEvent.setup();
  await openSearchTab(user);

  expect(await screen.findByText("処理中にエラーが発生しました。")).toBeInTheDocument();
  expect(screen.queryByText("種目が見つかりません")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "再試行" }));
  expect(await screen.findByText("懸垂")).toBeInTheDocument();
  expect(screen.queryByText("処理中にエラーが発生しました。")).not.toBeInTheDocument();
  expect(calls).toBe(2);
});

it("続きの取得に失敗しても表示済みの一覧を保ち、続きだけを再試行できる", async () => {
  const all = Array.from({ length: 55 }, (_, index) => {
    const number = String(index + 1).padStart(2, "0");
    return summary(`exercise_${number}`, `種目${number}`);
  });
  let moreCalls = 0;
  mockSearchAPI(params => {
    const offset = Number(params.get("offset") ?? "0");
    if (offset > 0 && ++moreCalls === 1) return HttpResponse.error();
    return HttpResponse.json(page(all.slice(offset, offset + 50), all.length, offset));
  });
  const user = userEvent.setup();
  await openSearchTab(user);
  await screen.findByText("種目50");

  await user.click(screen.getByRole("button", { name: "さらに表示" }));
  expect(await screen.findByText(/種目を取得できませんでした/)).toBeInTheDocument();
  expect(screen.getByText("種目01")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "続きを再試行" }));
  expect(await screen.findByText("種目55")).toBeInTheDocument();
  expect(screen.queryByText(/種目を取得できませんでした/)).not.toBeInTheDocument();
});

it("読み込み中は「見つかりません」を表示せず、条件を変えた後は古い応答を反映しない", async () => {
  const slow = deferred<Response>();
  let slowFinished = false;
  mockSearchAPI(async params => {
    const muscle = params.get("muscle");
    if (muscle === "大胸筋") {
      const response = await slow.promise;
      slowFinished = true;
      return response;
    }
    if (muscle) return HttpResponse.json(page([summary("curl", "アームカール", "上腕二頭筋")]));
    return HttpResponse.json(page([summary("pull_up", "懸垂")]));
  });
  const user = userEvent.setup();
  await openSearchTab(user);
  await screen.findByText("懸垂");
  await screen.findByRole("option", { name: "胸" });

  await user.selectOptions(screen.getByRole("combobox", { name: "大まかな部位" }), "胸");
  expect(await screen.findByText("種目を読み込み中...")).toBeInTheDocument();
  expect(screen.queryByText("種目が見つかりません")).not.toBeInTheDocument();

  await user.selectOptions(screen.getByRole("combobox", { name: "大まかな部位" }), "腕");
  expect(await screen.findByText("アームカール")).toBeInTheDocument();

  await act(async () => {
    slow.resolve(HttpResponse.json(page([summary("old", "古い結果", "大胸筋")])));
    await slow.promise;
  });
  await waitFor(() => expect(slowFinished).toBe(true));
  expect(screen.queryByText("古い結果")).not.toBeInTheDocument();
  expect(screen.getByText("アームカール")).toBeInTheDocument();
});

it("一致する種目がないときだけ「見つかりません」を表示する", async () => {
  mockSearchAPI(() => HttpResponse.json(page([])));
  const user = userEvent.setup();
  await openSearchTab(user);

  expect(await screen.findByText("種目が見つかりません")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "さらに表示" })).not.toBeInTheDocument();
});

it("選択肢の取得に失敗しても種目名での検索は使え、選択肢だけを再取得できる", async () => {
  let optionCalls = 0;
  mockSearchAPI(() => HttpResponse.json(page([summary("pull_up", "懸垂")])));
  server.use(http.get(`${apiOrigin}/api/exercises/filters`, () => {
    optionCalls++;
    return optionCalls === 1 ? HttpResponse.error() : HttpResponse.json(filterOptions);
  }));
  const user = userEvent.setup();
  await openSearchTab(user);

  expect(await screen.findByText("部位・器具の選択肢を取得できませんでした。")).toBeInTheDocument();
  expect(await screen.findByText("懸垂")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "選択肢を再取得" }));
  expect(await screen.findByRole("option", { name: "背中" })).toBeInTheDocument();
  expect(screen.queryByText("部位・器具の選択肢を取得できませんでした。")).not.toBeInTheDocument();
});
