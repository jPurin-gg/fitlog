import React from "react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";

import { CalendarWorkoutEditor } from "@/features/calendar/CalendarPage";

const apiOrigin = "http://localhost:3000";
const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it.each([
  ["回数", "10.5", "1セット目: 回数は小数ではなく整数で入力してください。"],
  ["回数", "1001", "1セット目: 回数は1〜1,000回の整数で入力してください。"],
  ["重量 kg", "2500", "1セット目: 重量は0〜2,000kgの範囲で入力してください。"],
])("過去記録の%sに %s を入れると保存せずにエラーを出す", async (label, value, message) => {
  let puts = 0;
  const onSaved = vi.fn();
  server.use(
    http.get(`${apiOrigin}/api/workouts/by-date/2026-11-12`, () => HttpResponse.json({
      workout_id: 5,
      date: "2026-11-12",
      title: "筋トレ",
      sets: [{ id: 1, exercise_id: "bench", exercise_name: "ベンチプレス", weight: 80, reps: 10, set_order: 1, feeling: "" }],
    })),
    http.put(`${apiOrigin}/api/workouts/by-date/2026-11-12`, () => { puts++; return HttpResponse.json({}); }),
  );
  const user = userEvent.setup();
  render(<CalendarWorkoutEditor date="2026-11-12" onClose={() => undefined} onSaved={onSaved} />);

  await user.clear(await screen.findByLabelText(label));
  await user.type(screen.getByLabelText(label), value);
  await user.click(screen.getByRole("button", { name: "保存" }));

  expect(await screen.findByText(message)).toBeInTheDocument();
  expect(puts).toBe(0);
  expect(onSaved).not.toHaveBeenCalled();
});
