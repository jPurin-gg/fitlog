import React from "react";
import { render, screen, within } from "@testing-library/react";
import { expect, it } from "vitest";
import { WeeklyActivity } from "./WeeklyActivity";

const dates = ["2026-11-07", "2026-11-08", "2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"];

function bars() {
  return within(screen.getByRole("list", { name: "直近7日のセット数" })).getAllByRole("listitem");
}

it("直近7日のセット数を日付付きのバーで表示し、今週の実施と予定を並べる", () => {
  render(<WeeklyActivity data={{
    chart_data: [4, 0, 0, 0, 8, 0, 2],
    chart_dates: dates,
    this_week: { week_start: "2026-11-09", workout_count: 2, planned_count: 3 },
  }} />);

  expect(screen.getByTestId("weekly-progress")).toHaveTextContent("今週 2 回 / 予定 3 回");
  const items = bars();
  expect(items).toHaveLength(7);
  expect(items[0]).toHaveAccessibleName("11月7日（土） 4セット");
  expect(items[4]).toHaveAccessibleName("11月11日（水） 8セット");
  expect(items[6]).toHaveAccessibleName("11月13日（金） 2セット");
  const heights = screen.getAllByTestId("weekly-bar").map(bar => bar.style.height);
  expect(heights[4]).toBe("100%");
  expect(heights[0]).toBe("50%");
  expect(heights[6]).toBe("25%");
  expect(screen.getByText("今日")).toBeInTheDocument();
  expect(screen.getByText("7日間で合計 14 セット記録しました。")).toBeInTheDocument();
});

it("記録0件でも7日分の枠と案内を表示し、予定0回も崩さず出す", () => {
  render(<WeeklyActivity data={{
    chart_data: [0, 0, 0, 0, 0, 0, 0],
    chart_dates: dates,
    this_week: { week_start: "2026-11-09", workout_count: 0, planned_count: 0 },
  }} />);

  expect(bars()).toHaveLength(7);
  for (const bar of screen.getAllByTestId("weekly-bar")) {
    expect(bar.style.height).toBe("");
  }
  expect(screen.getByTestId("weekly-progress")).toHaveTextContent("今週 0 回 / 予定 0 回");
  expect(screen.getByText("直近7日の記録はまだありません。セットを保存するとここに表示されます。")).toBeInTheDocument();
});

it("古い応答や不正な値でも7日分の枠を保ち、今週の回数は出さない", () => {
  render(<WeeklyActivity data={{ chart_data: [1, 2], this_week: { workout_count: "2" } }} />);

  expect(bars()).toHaveLength(7);
  expect(bars()[6]).toHaveAccessibleName("今日 0セット");
  expect(screen.queryByTestId("weekly-progress")).not.toBeInTheDocument();
  expect(screen.getByText("直近7日の記録はまだありません。セットを保存するとここに表示されます。")).toBeInTheDocument();
});

it("読み込み中は読み込み表示だけを出す", () => {
  render(<WeeklyActivity data={null} />);
  expect(screen.getByText("記録量を読み込み中...")).toBeInTheDocument();
  expect(screen.queryByRole("list")).not.toBeInTheDocument();
});
