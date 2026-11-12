import React from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { ExerciseSelectorModal } from "./ExerciseSelectorModal";
import { expectAccessible } from "@/test/a11y";

const origin = "http://localhost:3000";
const bench = { id: "bench", name: "ベンチプレス", equipment: "バーベル", primaryMuscles: ["大胸筋"] };
const squat = { id: "squat", name: "スクワット", equipment: "バーベル", primaryMuscles: ["大腿四頭筋"] };
const server = setupServer(
  http.get(`${origin}/api/exercises/favorites`, () => HttpResponse.json([bench])),
  http.get(`${origin}/api/exercises/recent`, () => HttpResponse.json([squat])),
  http.get(`${origin}/api/exercises`, () => HttpResponse.json([bench, squat])),
);
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it("お気に入り・最近・検索を切り替え、選んだ種目のIDと名前を渡す", async () => {
  const onSelect = vi.fn();
  render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={onSelect} />);
  await screen.findByRole("button", { name: /ベンチプレス/ });
  expect(screen.queryByRole("button", { name: /スクワット/ })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "最近" }));
  await userEvent.click(await screen.findByRole("button", { name: /スクワット/ }));
  expect(onSelect).toHaveBeenCalledWith({ id: "squat", name: "スクワット" });
  await userEvent.click(screen.getByRole("button", { name: "検索" }));
  expect(await screen.findByRole("button", { name: /ベンチプレス/ })).toBeInTheDocument();
  await expectAccessible(screen.getByRole("dialog", { name: "種目を選択" }));
});

it("部位・筋肉・器具・検索名をAPIへ送り、親部位の変更で細かい筋肉をリセットする", async () => {
  const queries: URLSearchParams[] = [];
  server.use(http.get(`${origin}/api/exercises`, ({ request }) => {
    queries.push(new URL(request.url).searchParams);
    return HttpResponse.json([]);
  }));
  render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={vi.fn()} />);
  await userEvent.type(screen.getByRole("textbox", { name: "種目名で検索" }), "プレス");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "大まかな部位" }), "腕");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "細かい筋肉" }), "上腕三頭筋");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "器具" }), "ダンベル");
  await waitFor(() => expect(queries.at(-1)?.get("equipment")).toBe("ダンベル"));
  expect(queries.at(-1)?.get("name")).toBe("プレス");
  expect(queries.at(-1)?.get("muscle")).toBe("上腕三頭筋");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "大まかな部位" }), "脚");
  expect(screen.getByRole("combobox", { name: "細かい筋肉" })).toHaveValue("");
  expect(screen.getByRole("option", { name: "ハムストリングス" })).toBeInTheDocument();
  expect(screen.queryByRole("option", { name: "上腕三頭筋" })).not.toBeInTheDocument();
  await waitFor(() => expect(queries.at(-1)?.get("muscle")).toBe("大腿四頭筋"));
});

it("お気に入りを解除・追加すると一覧に反映する", async () => {
  const methods: string[] = [];
  server.use(http.all(`${origin}/api/exercises/bench/favorite`, ({ request }) => {
    methods.push(request.method);
    return new HttpResponse(null, { status: 204 });
  }));
  render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={vi.fn()} />);
  await userEvent.click(await screen.findByRole("button", { name: "お気に入りから外す" }));
  expect(await screen.findByText(/まだお気に入りがありません/)).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "検索" }));
  const selection = await screen.findByRole("button", { name: /ベンチプレス/ });
  await userEvent.click(within(selection.parentElement!).getByRole("button", { name: "お気に入りに追加" }));
  await userEvent.click(screen.getByRole("button", { name: "お気に入り" }));
  expect(await screen.findByRole("button", { name: /ベンチプレス/ })).toBeInTheDocument();
  expect(methods).toEqual(["DELETE", "PUT"]);
});

it("カスタム種目を追加してID付きで選択し、キャンセルもできる", async () => {
  let body: unknown;
  const onSelect = vi.fn();
  server.use(http.post(`${origin}/api/exercises`, async ({ request }) => {
    body = await request.json();
    return HttpResponse.json({ id: "custom", name: "自分のプレス" });
  }));
  render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={onSelect} />);
  await userEvent.click(screen.getByRole("button", { name: "見つからない種目を新しく追加" }));
  expect(screen.getByRole("button", { name: "追加して選択" })).toBeDisabled();
  await expectAccessible(screen.getByRole("dialog"));
  await userEvent.click(screen.getByRole("button", { name: "キャンセル" }));
  expect(screen.getByRole("textbox", { name: "種目名で検索" })).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "見つからない種目を新しく追加" }));
  await userEvent.type(screen.getByRole("textbox", { name: /種目名/ }), "自分のプレス");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "カテゴリ（任意）" }), "その他");
  await userEvent.click(screen.getByRole("button", { name: "追加して選択" }));
  await waitFor(() => expect(onSelect).toHaveBeenCalledWith({ id: "custom", name: "自分のプレス" }));
  expect(body).toEqual({ name: "自分のプレス", category: "その他", equipment: "その他", primary_muscle: "" });
});
