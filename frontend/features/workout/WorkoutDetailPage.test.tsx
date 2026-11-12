import React from "react";
import { render, screen } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import WorkoutDetailPage from "./WorkoutDetailPage";
import { expectAccessible } from "@/test/a11y";

vi.mock("next/navigation", () => ({ useParams: () => ({ id: "42" }) }));
const endpoint = "http://localhost:3000/api/workouts/42";
const summary = {
  total_sets: 3, total_reps: 24, total_volume: 1440, duration_min: 30, pr_count: 1,
  ai_comment: "よく頑張りました。",
  exercises: [{ exercise_id: "bench", name: "ベンチプレス", sets: 3, total_reps: 24, best_weight: 60, total_volume: 1440 }],
};
const workout = { id: 42, title: "胸の日", status: "completed", started_at: "2026-10-01T10:00:00Z", ended_at: "2026-10-01T10:30:00Z", summary };
const server = setupServer(http.get(endpoint, () => HttpResponse.json(workout)));
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => { vi.spyOn(console, "error").mockImplementation(() => {}); });
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it("読み込み後に履歴の集計と種目・AI総評を表示する", async () => {
  render(<WorkoutDetailPage />);
  expect(screen.getByRole("heading", { name: "読み込み中" })).toBeInTheDocument();
  await screen.findByRole("heading", { name: "胸の日" });
  expect(screen.getByRole("heading", { name: "ベンチプレス" })).toBeInTheDocument();
  expect(screen.getByText("60kg")).toBeInTheDocument();
  expect(screen.getByText("24回")).toBeInTheDocument();
  expect(screen.getByText("よく頑張りました。")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "ホームへ戻る" })).toHaveAttribute("href", "/");
  await expectAccessible(screen.getByRole("main"));
});

it("進行中でセットがない履歴も表示できる", async () => {
  server.use(http.get(endpoint, () => HttpResponse.json({ ...workout, status: "active", ended_at: "", summary: { ...summary, ai_comment: "", exercises: [] } })));
  render(<WorkoutDetailPage />);
  await screen.findByRole("heading", { name: "胸の日" });
  expect(screen.getByText("進行中")).toBeInTheDocument();
  expect(screen.getByText(/から継続中/)).toBeInTheDocument();
  expect(screen.getByText("セット記録はありません。")).toBeInTheDocument();
  expect(screen.queryByText("AIコーチの総評")).not.toBeInTheDocument();
});

it("履歴取得の失敗を集計の代わりに表示する", async () => {
  server.use(http.get(endpoint, () => HttpResponse.json({ detail: "履歴が見つかりません。" }, { status: 404 })));
  render(<WorkoutDetailPage />);
  expect(await screen.findByRole("alert")).toHaveTextContent("履歴が見つかりません。");
  expect(screen.queryByRole("heading", { name: "種目別" })).not.toBeInTheDocument();
});
