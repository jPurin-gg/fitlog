import React from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import CalendarPage from "./CalendarPage";

const API = "http://localhost:3000";
const server = setupServer(
  http.get(`${API}/api/monthly-plans`, () => HttpResponse.json([])),
  http.get(`${API}/api/monthly-plans/:month`, () => HttpResponse.json({
    plan_month: "2026-11",
    plan_name: "PPL",
    frequency: "週3回",
    description: "",
    rationale: "",
    recommended_days: [1, 3, 5],
    weekly_routine: [
      { day_name: "Day 1", target: "Push (胸・肩・三頭)", example_exercises: [] },
      { day_name: "Day 2", target: "Pull (背中・二頭)", example_exercises: [] },
      { day_name: "Day 3", target: "Legs (脚・腹)", example_exercises: [] },
    ],
  })),
  http.get(`${API}/api/calendar`, () => HttpResponse.json({
    worked_out_dates: [10, 13],
    worked_out_days: [
      { date: 10, workout_id: 7, type: "とても長い名前の上半身トレーニング" },
      { date: 13, workout_id: 8, type: "胸の日" },
    ],
    planned_days: [],
  })),
  http.get(`${API}/api/workouts/by-date/:date`, () => HttpResponse.json({ workout_id: 8, date: "2026-11-13", title: "胸の日", sets: [] })),
);
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 13, 12));
});
afterEach(() => { server.resetHandlers(); vi.useRealTimers(); });
afterAll(() => server.close());

it("セル内は日付と点だけにし、10px未満の文字やはみ出す要素を置かない", async () => {
  render(<CalendarPage />);
  const cells = await screen.findAllByTestId("calendar-day-cell");
  await screen.findByRole("button", { name: /^2026-11-10 実施済み/ });
  expect(cells).toHaveLength(30);
  for (const cell of cells) {
    // jsdomはレイアウトを計算しないため、クラスで確認する。
    expect(cell).toHaveClass("overflow-hidden", "min-w-0", "aspect-square");
    for (const element of [cell, ...cell.querySelectorAll("*")]) {
      const className = element.getAttribute("class") || "";
      for (const [, size] of className.matchAll(/text-\[(\d+)px\]/g)) {
        expect(Number(size)).toBeGreaterThanOrEqual(10);
      }
      expect(className).not.toMatch(/line-clamp/);
    }
    // 狭い幅で表示されるラベルは1行で省略し、md未満では点だけを出す。
    for (const label of cell.querySelectorAll(".md\\:block")) {
      expect(label).toHaveClass("hidden", "truncate", "max-w-full");
    }
  }
});

it("狭い幅では余白と間隔を詰めてセルを広げる", async () => {
  render(<CalendarPage />);
  const [cell] = await screen.findAllByTestId("calendar-day-cell");
  const grid = cell.closest("button")!.parentElement!;
  expect(grid).toHaveClass("grid-cols-7", "gap-1");
  expect(grid).not.toHaveClass("gap-2");
  // 360px幅: 360 - 12*2（ページ） - 8*2（カード） - 4*6（間隔） = 296px → 1セル約42px。
  expect(grid.parentElement).toHaveClass("px-2");
  expect(grid.closest("main")!.parentElement).toHaveClass("px-3");
});

it("選んだ日の内容をグリッドの下に表示し、そこから編集を開く", async () => {
  render(<CalendarPage />);
  const panel = await screen.findByRole("region", { name: "選択した日の内容" });
  expect(await within(panel).findByText("実施済み: 胸の日")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /^2026-11-13 実施済み/ })).toHaveAttribute("aria-pressed", "true");

  await userEvent.click(screen.getByRole("button", { name: /^2026-11-10 実施済み/ }));
  expect(within(panel).getByText("実施済み: とても長い名前の上半身トレーニング")).toBeInTheDocument();
  expect(within(panel).getByRole("link", { name: /詳細/ })).toHaveAttribute("href", "/workouts/7");

  await userEvent.click(screen.getByRole("button", { name: /^2026-11-16 予定/ }));
  expect(within(panel).getByText("予定: 押す日")).toBeInTheDocument();
  expect(within(panel).getByRole("button", { name: "2026-11-16 の予定を編集" })).toHaveClass("min-h-11");

  await userEvent.click(screen.getByRole("button", { name: /^2026-11-13 実施済み/ }));
  await userEvent.click(within(panel).getByRole("button", { name: "2026-11-13 の記録を編集" }));
  expect(await screen.findByRole("heading", { name: "記録を編集" })).toBeInTheDocument();
});

it("別の月を表示中は未選択として案内する", async () => {
  render(<CalendarPage />);
  await screen.findByRole("button", { name: /^2026-11-13 実施済み/ });
  await userEvent.click(screen.getByRole("button", { name: "次の月" }));
  const panel = screen.getByRole("region", { name: "選択した日の内容" });
  expect(within(panel).getByText(/日付を選ぶと/)).toBeInTheDocument();
});
