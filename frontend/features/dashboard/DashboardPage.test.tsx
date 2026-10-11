import React from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AISettingsProvider } from "@/components/AISettings";
import DashboardPage from "./DashboardPage";

const auth = vi.hoisted(() => ({ user: { id: 1, nickname: "テスト" } }));
vi.mock("@/components/AuthGate", () => ({ useAuth: () => auth, LogoutButton: () => null }));

let monthlyCalls = 0;
const server = setupServer(
  http.get("http://localhost:3000/api/dashboard", () => HttpResponse.json({ recent_workouts: [] })),
  http.get("http://localhost:3000/api/preferences", () => HttpResponse.json({})),
  http.get("http://localhost:3000/api/monthly-plans/:month", () => {
    monthlyCalls++;
    return HttpResponse.json({ error: "not found" }, { status: 404 });
  }),
);
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-01T12:00:00Z"));
  auth.user.id = 1;
  monthlyCalls = 0;
});
afterEach(() => { server.resetHandlers(); vi.useRealTimers(); vi.restoreAllMocks(); });
afterAll(() => server.close());

function show() {
  return render(<AISettingsProvider userID={auth.user.id}><DashboardPage /></AISettingsProvider>);
}

it("未作成の月の最初のホーム訪問で案内し、AI OFFのまま記録に進める", async () => {
  show();
  const dialog = await screen.findByRole("dialog", { name: "月間プラン" });
  expect(within(dialog).getByRole("switch")).not.toBeChecked();
  expect(within(dialog).getByRole("button", { name: "決定してプランを作成" })).toBeDisabled();
  expect(within(dialog).getByRole("link", { name: "プランなしで記録を始める" })).toHaveAttribute("href", "/workout");
});

it("今月の非表示は再訪後も効き、手動で開いて閉じても解除しない", async () => {
  const first = show();
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "今月は次回以降表示しない" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  first.unmount();
  const second = show();
  await waitFor(() => expect(monthlyCalls).toBe(2));
  await screen.findByText("まだ完了したワークアウトはありません。");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "月間プランを作成／見直す" }));
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "閉じる" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  second.unmount();
  show();
  await waitFor(() => expect(monthlyCalls).toBe(3));
  await screen.findByText("まだ完了したワークアウトはありません。");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("通常の閉じるは今回だけで、次のホーム訪問では案内する", async () => {
  const first = show();
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "閉じる" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  first.unmount();
  show();
  await screen.findByRole("dialog");
});

it("非表示にしたホームを開いたまま翌月になっても再び案内する", async () => {
  show();
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("button", { name: "今月は次回以降表示しない" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  act(() => {
    vi.setSystemTime(new Date("2026-11-01T12:00:00Z"));
    window.dispatchEvent(new Event("focus"));
  });
  await screen.findByRole("dialog");
  expect(monthlyCalls).toBe(2);
});

it("別ユーザーには今月の非表示を引き継がない", async () => {
  window.localStorage.setItem("fitlog:monthly-prompt-dismissed:1", "2026-10");
  auth.user.id = 2;
  show();
  await screen.findByRole("dialog");
});

it("今月のプランがあるときは自動で案内しない", async () => {
  server.use(http.get("http://localhost:3000/api/monthly-plans/:month", () => HttpResponse.json({ recommended_days: [], weekly_routine: [] })));
  show();
  await screen.findByText("休息日");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "月間プランを作成／見直す" }));
  await screen.findByRole("dialog");
});

it("プランの読み込み失敗を未作成と誤認して案内しない", async () => {
  const error = vi.spyOn(console, "error").mockImplementation(() => {});
  server.use(http.get("http://localhost:3000/api/monthly-plans/:month", () => HttpResponse.json({ error: "unavailable" }, { status: 500 })));
  show();
  await waitFor(() => expect(error).toHaveBeenCalled());
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "プランなしで記録を始める" })).toHaveAttribute("href", "/workout");
});
