import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";

import WorkoutDetailPage from "./WorkoutDetailPage";

const navigation = vi.hoisted(() => ({ replace: vi.fn() }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "42" }),
  useRouter: () => ({ replace: navigation.replace }),
}));

const apiOrigin = "http://localhost:3000";
const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => navigation.replace.mockClear());
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

const detail = {
  id: 42,
  title: "胸の日",
  started_at: new Date(2026, 7, 24, 19, 30).toISOString(),
  ended_at: new Date(2026, 7, 24, 20, 17).toISOString(),
  status: "completed",
  summary: {
    total_sets: 3,
    total_reps: 26,
    total_volume: 1180,
    duration_min: 47,
    pr_count: 1,
    exercises: [
      { exercise_id: "bench", name: "ベンチプレス", sets: 2, total_reps: 18, best_weight: 60, total_volume: 1080 },
      { exercise_id: "fly", name: "ダンベルフライ", sets: 1, total_reps: 8, best_weight: 12.5, total_volume: 100 },
    ],
  },
  sets: [
    { id: 101, exercise_id: "bench", exercise_name: "ベンチプレス", set_order: 1, weight: 60, reps: 10, feeling: "余裕あり", is_pr: true, created_at: new Date(2026, 7, 24, 19, 35).toISOString() },
    { id: 102, exercise_id: "bench", exercise_name: "ベンチプレス", set_order: 2, weight: 60, reps: 8, feeling: "", is_pr: false, created_at: new Date(2026, 7, 24, 19, 40).toISOString() },
    { id: 103, exercise_id: "fly", exercise_name: "ダンベルフライ", set_order: 1, weight: 12.5, reps: 8, feeling: "肩に違和感", is_pr: false, created_at: new Date(2026, 7, 24, 20, 17).toISOString() },
  ],
};

it("セットごとの重量・回数・感想を種目ごとに表示する", async () => {
  server.use(http.get(`${apiOrigin}/api/workouts/42`, () => HttpResponse.json(detail)));

  render(<WorkoutDetailPage />);

  const benchSets = within(await screen.findByRole("list", { name: "ベンチプレスのセット" })).getAllByRole("listitem");
  expect(benchSets).toHaveLength(2);
  expect(within(benchSets[0]).getByText("1セット目")).toBeInTheDocument();
  expect(within(benchSets[0]).getByText("60kg × 10回")).toBeInTheDocument();
  expect(within(benchSets[0]).getByText("感想: 余裕あり")).toBeInTheDocument();
  expect(within(benchSets[0]).getByText("自己ベスト")).toBeInTheDocument();
  expect(within(benchSets[1]).getByText("2セット目")).toBeInTheDocument();
  expect(within(benchSets[1]).getByText("60kg × 8回")).toBeInTheDocument();
  expect(within(benchSets[1]).getByText("感想なし")).toBeInTheDocument();
  expect(within(benchSets[1]).queryByText("自己ベスト")).not.toBeInTheDocument();

  const flySets = within(screen.getByRole("list", { name: "ダンベルフライのセット" })).getAllByRole("listitem");
  expect(flySets).toHaveLength(1);
  expect(within(flySets[0]).getByText("12.5kg × 8回")).toBeInTheDocument();
  expect(within(flySets[0]).getByText("感想: 肩に違和感")).toBeInTheDocument();
  expect(screen.getByText(/8月24日\(月\)\s*19:30 から/)).toBeInTheDocument();
});

it("確認してから記録を削除し、履歴一覧へ移動する", async () => {
  let deleteRequests = 0;
  server.use(
    http.get(`${apiOrigin}/api/workouts/42`, () => HttpResponse.json(detail)),
    http.delete(`${apiOrigin}/api/workouts/42`, () => {
      deleteRequests++;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  const user = userEvent.setup();
  render(<WorkoutDetailPage />);

  // 確認を取り消したときは削除しない。
  await user.click(await screen.findByRole("button", { name: "この記録を削除" }));
  await user.click(within(screen.getByRole("dialog", { name: "記録の削除" })).getByRole("button", { name: "キャンセル" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(deleteRequests).toBe(0);

  await user.click(screen.getByRole("button", { name: "この記録を削除" }));
  await user.click(within(screen.getByRole("dialog", { name: "記録の削除" })).getByRole("button", { name: "削除する" }));

  await waitFor(() => expect(navigation.replace).toHaveBeenCalledWith("/workouts"));
  expect(deleteRequests).toBe(1);
});

it("削除に失敗したら理由を表示し、画面にとどまる", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  server.use(
    http.get(`${apiOrigin}/api/workouts/42`, () => HttpResponse.json(detail)),
    http.delete(`${apiOrigin}/api/workouts/42`, () => HttpResponse.json({ status: 500, detail: "処理中にエラーが発生しました。" }, { status: 500 })),
  );
  const user = userEvent.setup();
  render(<WorkoutDetailPage />);

  await user.click(await screen.findByRole("button", { name: "この記録を削除" }));
  const dialog = screen.getByRole("dialog", { name: "記録の削除" });
  await user.click(within(dialog).getByRole("button", { name: "削除する" }));

  expect(await within(dialog).findByRole("alert")).toHaveTextContent("処理中にエラーが発生しました。");
  expect(within(dialog).getByRole("button", { name: "削除する" })).toBeEnabled();
  expect(navigation.replace).not.toHaveBeenCalled();
});
