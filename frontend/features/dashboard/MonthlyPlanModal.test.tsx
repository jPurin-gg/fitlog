import React from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AISettingsProvider } from "@/components/AISettings";
import { MonthlyPlanModal } from "./DashboardPage";

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function show(onPlanGenerated = vi.fn()) {
  render(<AISettingsProvider userID={1}><MonthlyPlanModal onClose={vi.fn()} onPlanGenerated={onPlanGenerated} /></AISettingsProvider>);
}

it("案内を開くだけでは生成せず、ONにして明示的に作成できる", async () => {
  const onPlanGenerated = vi.fn();
  let calls = 0;
  const plan = { plan_name: "今月のプラン" };
  server.use(http.post("http://localhost:3000/api/monthly-plans/:month/generate", () => {
    calls++;
    return HttpResponse.json(plan);
  }));
  show(onPlanGenerated);
  expect(screen.getByRole("button", { name: "決定してプランを作成" })).toBeDisabled();
  expect(calls).toBe(0);
  await userEvent.click(screen.getByRole("switch"));
  expect(calls).toBe(0);
  await userEvent.click(screen.getByRole("button", { name: "決定してプランを作成" }));
  await waitFor(() => expect(onPlanGenerated).toHaveBeenCalledWith(plan));
  expect(calls).toBe(1);
});

it("生成中にOFF→ONにしても古いプランを反映せず、再生成しない", async () => {
  let resolve!: (response: Response) => void;
  const pending = new Promise<Response>(next => { resolve = next; });
  const onPlanGenerated = vi.fn();
  let calls = 0;
  let finished = false;
  server.use(http.post("http://localhost:3000/api/monthly-plans/:month/generate", async () => {
    calls++;
    const response = await pending;
    finished = true;
    return response;
  }));
  show(onPlanGenerated);
  const user = userEvent.setup();
  await user.click(screen.getByRole("switch"));
  await user.click(screen.getByRole("button", { name: "決定してプランを作成" }));
  await waitFor(() => expect(calls).toBe(1));
  await user.click(screen.getByRole("switch"));
  await user.click(screen.getByRole("switch"));
  await act(async () => { resolve(HttpResponse.json({ plan_name: "古いプラン" })); await pending; });
  await waitFor(() => expect(finished).toBe(true));
  expect(onPlanGenerated).not.toHaveBeenCalled();
  expect(calls).toBe(1);
  expect(screen.getByRole("button", { name: "決定してプランを作成" })).toBeEnabled();
});
