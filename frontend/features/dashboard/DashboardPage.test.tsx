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

function existingPlan() {
  server.use(http.get("http://localhost:3000/api/monthly-plans/:month", () => HttpResponse.json({ recommended_days: [], weekly_routine: [] })));
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(done => { resolve = done; });
  return { promise, resolve };
}

it("記録の取得が500なら読み込みを終え、再試行で記録を表示する", async () => {
  existingPlan();
  const response = deferred<Response>();
  let calls = 0;
  server.use(http.get("http://localhost:3000/api/dashboard", () => {
    calls++;
    return calls === 1
      ? HttpResponse.json({ detail: "記録を取得できませんでした。" }, { status: 500 })
      : response.promise;
  }));
  show();
  const error = await screen.findByRole("alert");
  expect(error).toHaveTextContent("記録を取得できませんでした。");
  expect(screen.queryByText("記録を読み込み中...")).not.toBeInTheDocument();
  await userEvent.click(within(error).getByRole("button", { name: "再試行" }));
  expect(await screen.findByText("記録を読み込み中...")).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  response.resolve(HttpResponse.json({ recent_workouts: [{ id: 12, title: "胸の日", time: "昨日", duration: "30分" }] }));
  expect(await screen.findByRole("link", { name: /胸の日/ })).toHaveAttribute("href", "/workouts/12");
  expect(screen.queryByText("記録を読み込み中...")).not.toBeInTheDocument();
  expect(calls).toBe(2);
});

it("設定の取得前・失敗時は保存できず、開いたまま再試行すると保存済みの値を使える", async () => {
  existingPlan();
  const response = deferred<Response>();
  const savedPreferences = {
    preferred_equipment: ["ダンベル"],
    avoided_equipment: ["バーベル"],
    training_environment: "家",
    notes: "保存済みのメモ",
  };
  let calls = 0;
  let savedBody: unknown;
  server.use(
    http.get("http://localhost:3000/api/preferences", () => {
      calls++;
      return calls === 1 ? response.promise : HttpResponse.json(savedPreferences);
    }),
    http.put("http://localhost:3000/api/preferences", async ({ request }) => {
      savedBody = await request.json();
      return HttpResponse.json(savedPreferences);
    }),
  );
  show();
  await userEvent.click(screen.getByRole("button", { name: "トレーニング設定" }));
  const dialog = await screen.findByRole("dialog", { name: "トレーニング設定" });
  expect(within(dialog).getByText("設定を読み込み中...")).toBeInTheDocument();
  await userEvent.click(within(dialog).getByRole("button", { name: "保存する" }));
  expect(within(dialog).getByRole("button", { name: "保存する" })).toBeDisabled();
  expect(savedBody).toBeUndefined();

  response.resolve(HttpResponse.json({ detail: "設定を取得できませんでした。" }, { status: 500 }));
  const error = await within(dialog).findByRole("alert");
  expect(error).toHaveTextContent("設定を取得できませんでした。");
  expect(within(dialog).queryByText("設定を読み込み中...")).not.toBeInTheDocument();
  expect(within(dialog).getByRole("button", { name: "保存する" })).toBeDisabled();
  await userEvent.click(within(dialog).getByRole("button", { name: "保存する" }));
  expect(savedBody).toBeUndefined();

  await userEvent.click(within(error).getByRole("button", { name: "再試行" }));
  expect(await within(dialog).findByLabelText("メモ")).toHaveValue("保存済みのメモ");
  expect(within(dialog).getByRole("button", { name: "家" })).toHaveAttribute("aria-pressed", "true");
  expect(within(within(dialog).getByRole("group", { name: "優先する器具" })).getByRole("button", { name: "ダンベル" })).toHaveAttribute("aria-pressed", "true");
  expect(within(within(dialog).getByRole("group", { name: "避けたい器具" })).getByRole("button", { name: "バーベル" })).toHaveAttribute("aria-pressed", "true");
  expect(within(dialog).getByRole("button", { name: "保存する" })).toBeEnabled();
  await userEvent.click(within(dialog).getByRole("button", { name: "保存する" }));
  await waitFor(() => expect(savedBody).toEqual(savedPreferences));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
});

it("設定を取得前に開いても、最初の取得が成功したら保存済みの値が入る", async () => {
  existingPlan();
  const response = deferred<Response>();
  server.use(http.get("http://localhost:3000/api/preferences", () => response.promise));
  show();
  await userEvent.click(screen.getByRole("button", { name: "トレーニング設定" }));
  const dialog = await screen.findByRole("dialog", { name: "トレーニング設定" });
  expect(within(dialog).getByRole("button", { name: "保存する" })).toBeDisabled();
  response.resolve(HttpResponse.json({ notes: "先に保存していた値", training_environment: "どちらも" }));
  expect(await within(dialog).findByLabelText("メモ")).toHaveValue("先に保存していた値");
  expect(within(dialog).getByRole("button", { name: "どちらも" })).toHaveAttribute("aria-pressed", "true");
  expect(within(dialog).getByRole("button", { name: "保存する" })).toBeEnabled();
});

it("種目差し替えの保存失敗で元の名前とIDに戻り、再度変更して保存できる", async () => {
  const plan = {
    plan_month: "2026-10",
    plan_name: "月間プラン",
    frequency: "週2回",
    description: "説明",
    rationale: "理由",
    rest_days: [],
    recommended_days: [4, 6],
    weekly_routine: [
      { day_name: "木曜日", target: "胸", example_exercises: ["ベンチプレス", "腕立て伏せ"], exercise_ids: ["bench", "pushup"] },
      { day_name: "土曜日", target: "脚", example_exercises: ["スクワット"], exercise_ids: ["squat"] },
    ],
  };
  const response = deferred<Response>();
  const payloads: typeof plan[] = [];
  server.use(
    http.get("http://localhost:3000/api/monthly-plans/:month", () => HttpResponse.json(plan)),
    http.post("http://localhost:3000/api/exercises", () => HttpResponse.json({ id: "dumbbell", name: "ダンベルプレス" })),
    http.put("http://localhost:3000/api/monthly-plans/:month", async ({ request }) => {
      const input = await request.json() as typeof plan;
      payloads.push(input);
      return payloads.length === 1 ? response.promise : HttpResponse.json({ ...input, plan_month: "2026-10" });
    }),
  );
  show();
  await screen.findByText("ベンチプレス");
  expect(screen.getByText(/種目の変更は、今月の月間プランの毎週のこの曜日/)).toBeInTheDocument();

  const replace = async () => {
    await userEvent.click(screen.getAllByRole("button", { name: "種目を変更・AIに相談" })[0]);
    const dialog = await screen.findByRole("dialog", { name: "種目を変更" });
    expect(within(dialog).getByText(/毎週のこの曜日/)).toBeInTheDocument();
    await userEvent.type(within(dialog).getByPlaceholderText("好きな種目名を入力"), "ダンベルプレス");
    await userEvent.click(within(dialog).getByRole("button", { name: "決定" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  };
  await replace();
  expect(screen.getByText("ダンベルプレス")).toBeInTheDocument();
  expect(screen.queryByText("ベンチプレス")).not.toBeInTheDocument();
  expect(screen.getByText("種目の変更を保存中...")).toBeInTheDocument();
  screen.getAllByRole("button", { name: "種目を変更・AIに相談" }).forEach(button => expect(button).toBeDisabled());
  await waitFor(() => expect(payloads).toHaveLength(1));
  expect(payloads[0].weekly_routine[0]).toEqual({ ...plan.weekly_routine[0], example_exercises: ["ダンベルプレス", "腕立て伏せ"], exercise_ids: ["dumbbell", "pushup"] });
  expect(payloads[0].weekly_routine[1]).toEqual(plan.weekly_routine[1]);

  response.resolve(HttpResponse.json({ detail: "プランを保存できませんでした。" }, { status: 500 }));
  expect(await screen.findByRole("alert")).toHaveTextContent("プランを保存できませんでした。 元の種目に戻しました。");
  expect(screen.getByText("ベンチプレス")).toBeInTheDocument();
  expect(screen.queryByText("ダンベルプレス")).not.toBeInTheDocument();
  expect(screen.queryByText("種目の変更を保存中...")).not.toBeInTheDocument();
  await userEvent.click(screen.getAllByRole("button", { name: "種目を変更・AIに相談" })[0]);
  const dialog = await screen.findByRole("dialog", { name: "種目を変更" });
  expect(within(dialog).getByText("ベンチプレス")).toBeInTheDocument();
  // 元のIDでAIへ相談できることも、実際のリクエストで確認する。
  let consultedID = "";
  server.use(http.post("http://localhost:3000/api/exercises/:id/alternatives", ({ params }) => {
    consultedID = String(params.id);
    return HttpResponse.json({ message: "候補", alternatives: [] });
  }));
  await userEvent.click(within(dialog).getByRole("switch"));
  await userEvent.click(within(dialog).getByRole("button", { name: "AIに代替種目を相談する" }));
  await waitFor(() => expect(consultedID).toBe("bench"));
  await within(dialog).findByText("候補");
  await userEvent.type(within(dialog).getByPlaceholderText("リストにない種目を手動で入力"), "ダンベルプレス");
  await userEvent.click(within(dialog).getByRole("button", { name: "決定" }));
  await waitFor(() => expect(payloads).toHaveLength(2));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
  expect(await screen.findByText("ダンベルプレス")).toBeInTheDocument();
  expect(screen.queryByText("ベンチプレス")).not.toBeInTheDocument();
});

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
