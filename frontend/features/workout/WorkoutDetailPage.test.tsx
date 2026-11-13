import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";

import WorkoutDetailPage from "./WorkoutDetailPage";

vi.mock("next/navigation", () => ({ useParams: () => ({ id: "42" }) }));

const apiOrigin = "http://localhost:3000";
const server = setupServer(
  http.get(`${apiOrigin}/api/workouts/42`, () => HttpResponse.json({
    id: 42,
    title: "胸の日の記録",
    // Pacific/Kiritimati（UTC+14）では8/25 06:30。実行環境（ブラウザ）のタイムゾーンと
    // 確実に異なるよう、あえて一般的でないタイムゾーンをサーバー側に設定する。
    started_at: "2026-08-24T16:30:00Z",
    ended_at: "2026-08-24T17:15:00Z",
    status: "completed",
    summary: { total_sets: 0, total_reps: 0, total_volume: 0, duration_min: 45, pr_count: 0, exercises: [] },
  })),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it("履歴の時刻はブラウザではなくサーバーが返すタイムゾーンで表示する", async () => {
  server.use(http.get(`${apiOrigin}/api/today`, () => HttpResponse.json({
    date: "2026-08-25", timezone: "Pacific/Kiritimati", rest_day: false, plan: null, workout: null,
  })));
  render(<WorkoutDetailPage />);

  expect(await screen.findByText(/8月25日.*06:30 から 8月25日.*07:15 まで/)).toBeInTheDocument();
});

it("タイムゾーンを取得できなくても履歴は表示する", async () => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  server.use(http.get(`${apiOrigin}/api/today`, () => HttpResponse.json({ status: 500 }, { status: 500 })));
  render(<WorkoutDetailPage />);

  expect(await screen.findByText(/ から .* まで/)).toBeInTheDocument();
});
