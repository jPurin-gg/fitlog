import React from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AISettingsProvider } from "@/components/AISettings";
import { AlternativeCoachModal } from "./AlternativeCoachModal";

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function show(onReplace = vi.fn()) {
  render(<AISettingsProvider userID={1}><AlternativeCoachModal exerciseId="bench" exerciseName="ベンチプレス" onClose={vi.fn()} onReplace={onReplace} /></AISettingsProvider>);
}

it("OFFでAI相談をせず、辞書の種目セレクターから手動で変更できる", async () => {
  const onReplace = vi.fn();
  server.use(
    http.get("http://localhost:3000/api/exercises/favorites", () => HttpResponse.json([])),
    http.get("http://localhost:3000/api/exercises/recent", () => HttpResponse.json([])),
    http.get("http://localhost:3000/api/exercises", () => HttpResponse.json([{ id: "bodyweight_squat", name: "自重スクワット", primaryMuscles: ["大腿四頭筋"] }])),
  );
  show(onReplace);
  expect(screen.getByRole("button", { name: "AIに代替種目を相談する" })).toBeDisabled();
  expect(screen.queryByRole("textbox", { name: /種目名/ })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "辞書から種目を選ぶ" }));
  await userEvent.type(screen.getByPlaceholderText("種目名で検索..."), "自重スクワット");
  await userEvent.click(await screen.findByRole("button", { name: /^自重スクワット/ }));
  await waitFor(() => expect(onReplace).toHaveBeenCalledWith({ id: "bodyweight_squat", name: "自重スクワット" }));
});

it("相談中のOFF→ONでも古い候補を表示しない", async () => {
  let resolve!: (response: Response) => void;
  const pending = new Promise<Response>(next => { resolve = next; });
  let calls = 0;
  let finished = false;
  server.use(http.post("http://localhost:3000/api/exercises/bench/alternatives", async () => {
    calls++;
    const response = await pending;
    finished = true;
    return response;
  }));
  show();
  const user = userEvent.setup();
  await user.click(screen.getByRole("switch"));
  await user.click(screen.getByRole("button", { name: "AIに代替種目を相談する" }));
  await waitFor(() => expect(calls).toBe(1));
  await user.click(screen.getByRole("switch"));
  await user.click(screen.getByRole("switch"));
  await act(async () => { resolve(HttpResponse.json({ message: "古い候補", alternatives: [] })); await pending; });
  await waitFor(() => expect(finished).toBe(true));
  expect(screen.queryByText("古い候補")).not.toBeInTheDocument();
  expect(calls).toBe(1);
  expect(screen.getByRole("button", { name: "AIに代替種目を相談する" })).toBeEnabled();
});
