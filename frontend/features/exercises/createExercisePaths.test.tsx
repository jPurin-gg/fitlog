import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AISettingsProvider } from "@/components/AISettings";
import ExercisesPage from "./ExercisesPage";
import { AlternativeCoachModal } from "./components/AlternativeCoachModal";
import { ExerciseSelectorModal } from "./components/ExerciseSelectorModal";

const api = "http://localhost:3000";
const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

const bench = {
  id: "bench_press", name: "ベンチプレス", category: "筋力トレーニング", equipment: "バーベル", level: "初級",
  force: "", mechanic: "", instructions: [], primaryMuscles: ["大胸筋"], secondaryMuscles: [], images: [],
};

let createdBodies: unknown[];

beforeEach(() => {
  createdBodies = [];
  server.use(
    http.get(`${api}/api/exercises/favorites`, () => HttpResponse.json([])),
    http.get(`${api}/api/exercises/recent`, () => HttpResponse.json([])),
    http.get(`${api}/api/exercises`, () => HttpResponse.json([bench])),
    http.post(`${api}/api/exercises`, async ({ request }) => {
      const body = await request.json() as { name: string };
      createdBodies.push(body);
      if (body.name.replace(/\s/g, "") === bench.name) {
        return HttpResponse.json(
          { status: 409, code: "CONFLICT", detail: "「ベンチプレス」は既に辞書にあります。新しく追加せず、既存の種目を選んでください。", existing: bench },
          { status: 409, headers: { "Content-Type": "application/problem+json" } },
        );
      }
      return HttpResponse.json({ ...bench, ...body, id: "custom_new", primaryMuscles: ["大胸筋"] }, { status: 201 });
    }),
  );
});

type User = ReturnType<typeof userEvent.setup>;

type Path = {
  // 追加フォームを開き、送信ボタンの名前を返す
  open: (user: User) => Promise<string>;
  // 既存の種目を選んだ結果を確認する
  expectExistingChosen: () => Promise<void>;
};

const paths: Record<string, () => Path> = {
  "種目辞書の「種目を追加」": () => ({
    open: async user => {
      render(<ExercisesPage />);
      await user.click(screen.getByRole("button", { name: "種目を追加" }));
      return "辞書に追加";
    },
    expectExistingChosen: async () => {
      // 追加モーダルを閉じ、既存の種目の詳細を開く
      expect(await screen.findByRole("heading", { level: 2, name: "ベンチプレス" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "辞書に追加" })).not.toBeInTheDocument();
    },
  }),
  "種目セレクターの「見つからない種目を新しく追加」": () => {
    const onSelect = vi.fn();
    return {
      open: async user => {
        render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={onSelect} />);
        await user.click(screen.getByRole("button", { name: "見つからない種目を新しく追加" }));
        return "追加して選択";
      },
      expectExistingChosen: async () => {
        await waitFor(() => expect(onSelect).toHaveBeenCalledWith({ id: "bench_press", name: "ベンチプレス" }));
      },
    };
  },
  "代替種目モーダルの手動変更": () => {
    const onReplace = vi.fn();
    return {
      open: async user => {
        render(<AISettingsProvider userID={1}><AlternativeCoachModal exerciseId="squat" exerciseName="スクワット" onClose={vi.fn()} onReplace={onReplace} /></AISettingsProvider>);
        await user.click(screen.getByRole("button", { name: "辞書から種目を選ぶ" }));
        await user.click(screen.getByRole("button", { name: "見つからない種目を新しく追加" }));
        return "追加して選択";
      },
      expectExistingChosen: async () => {
        await waitFor(() => expect(onReplace).toHaveBeenCalledWith({ id: "bench_press", name: "ベンチプレス" }));
      },
    };
  },
};

describe.each(Object.entries(paths))("%s", (_name, makePath) => {
  it("共通フォームで部位を必須にし、カタログ表記の分類で作成する", async () => {
    const user = userEvent.setup();
    const submit = await makePath().open(user);

    await user.clear(screen.getByLabelText(/種目名/));
    await user.type(screen.getByLabelText(/種目名/), "  インクラインダンベルプレス ");
    expect(screen.getByRole("button", { name: submit })).toBeDisabled();

    await user.selectOptions(screen.getByLabelText(/メイン部位/), "大胸筋");
    await user.click(screen.getByRole("button", { name: submit }));

    await waitFor(() => expect(createdBodies).toHaveLength(1));
    expect(createdBodies[0]).toEqual({
      name: "インクラインダンベルプレス",
      category: "筋力トレーニング",
      equipment: "その他",
      level: "初級",
      primary_muscles: ["大胸筋"],
      secondary_muscles: [],
      instructions: [],
    });
  });

  it("同名の種目があれば新規作成せず、既存の種目を選ばせる", async () => {
    const user = userEvent.setup();
    const path = makePath();
    const submit = await path.open(user);

    await user.clear(screen.getByLabelText(/種目名/));
    await user.type(screen.getByLabelText(/種目名/), "ベンチ プレス");
    await user.selectOptions(screen.getByLabelText(/メイン部位/), "大胸筋");
    await user.click(screen.getByRole("button", { name: submit }));

    expect(await screen.findByText(/既に辞書にあります/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /: ベンチプレス$/ }));

    await path.expectExistingChosen();
    expect(createdBodies).toHaveLength(1);
  });
});

it("種目セレクターの検索語を追加フォームの種目名に引き継ぐ", async () => {
  const user = userEvent.setup();
  render(<ExerciseSelectorModal onClose={vi.fn()} onSelect={vi.fn()} />);
  await user.type(screen.getByPlaceholderText("種目名で検索..."), "ケーブルクロス");
  await user.click(screen.getByRole("button", { name: "見つからない種目を新しく追加" }));
  expect(screen.getByLabelText(/種目名/)).toHaveValue("ケーブルクロス");
});
