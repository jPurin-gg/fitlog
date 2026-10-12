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

it("OFFでAI相談をせず、手動で種目を登録できる", async () => {
  const onReplace = vi.fn();
  server.use(http.post("http://localhost:3000/api/exercises", () => HttpResponse.json({ id: "manual", name: "自重スクワット" })));
  show(onReplace);
  expect(screen.getByRole("button", { name: "AIに代替種目を相談する" })).toBeDisabled();
  await userEvent.type(screen.getByPlaceholderText("好きな種目名を入力"), "自重スクワット");
  await userEvent.click(screen.getByRole("button", { name: "決定" }));
  await waitFor(() => expect(onReplace).toHaveBeenCalledWith({ id: "manual", name: "自重スクワット" }));
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
