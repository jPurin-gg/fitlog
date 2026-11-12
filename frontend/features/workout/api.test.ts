import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { getRecordedProgress, getTodayWorkoutStatus } from "./api";
import { ApiError } from "@/lib/api";

const apiOrigin = "http://localhost:3000";
const date = "2026-11-12";
const byDateURL = `${apiOrigin}/api/workouts/by-date/${date}`;
const detailURL = `${apiOrigin}/api/workouts/42`;
const server = setupServer();
const sets = [
  { id: 1, exercise_id: "bench", exercise_name: "ベンチプレス", set_order: 1, weight: 60, reps: 8 },
  { id: 2, exercise_id: "bench", exercise_name: "ベンチプレス", set_order: 2, weight: 60, reps: 8 },
];

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function datedWorkout(workoutID = 42) {
  return { workout_id: workoutID, date, title: "今日の記録", sets };
}

function detail(totalSets = 0, status = "active") {
  return { id: 42, status, summary: { total_sets: totalSets, exercises: [] } };
}

function notFound() {
  return HttpResponse.json({ status: 404, detail: "記録がありません。" }, { status: 404 });
}

describe("当日の記録の状態", () => {
  it("進行中の記録と保存済みセット数を返す", async () => {
    server.use(
      http.get(byDateURL, () => HttpResponse.json(datedWorkout())),
      http.get(detailURL, () => HttpResponse.json(detail(2))),
    );
    await expect(getTodayWorkoutStatus(date)).resolves.toEqual({ state: "active", workoutID: 42, setCount: 2 });
  });

  it("完了済みの記録のIDを返す", async () => {
    server.use(
      http.get(byDateURL, () => HttpResponse.json(datedWorkout())),
      http.get(detailURL, () => HttpResponse.json(detail(2, "completed"))),
    );
    await expect(getTodayWorkoutStatus(date)).resolves.toEqual({ state: "completed", workoutID: 42 });
  });

  it("日付指定APIの404だけを未開始として扱う", async () => {
    server.use(http.get(byDateURL, notFound));
    await expect(getTodayWorkoutStatus(date)).resolves.toEqual({ state: "none" });

    server.use(http.get(byDateURL, () => HttpResponse.json({ status: 500 }, { status: 500 })));
    await expect(getTodayWorkoutStatus(date)).rejects.toBeInstanceOf(ApiError);
  });
});

describe("開始APIが返したワークアウトの復元", () => {
  it("同じIDのセット内訳から番号と一覧を復元する", async () => {
    server.use(http.get(byDateURL, () => HttpResponse.json(datedWorkout())));
    const progress = await getRecordedProgress(42, date);
    expect(progress.exercises[0]).toMatchObject({ sets, setCount: 2, nextSet: 3 });
  });

  it.each(["別の記録", "404"])("日付指定が%sなら、ID指定で0件を確認した場合だけ空で開始できる", async result => {
    server.use(
      http.get(byDateURL, () => result === "404" ? notFound() : HttpResponse.json(datedWorkout(99))),
      http.get(detailURL, () => HttpResponse.json(detail())),
    );
    await expect(getRecordedProgress(42, date)).resolves.toEqual({ exercises: [], lastExerciseID: null });
  });

  it("日付指定が別の記録で集計に記録がある場合は不正確な番号で再開しない", async () => {
    server.use(
      http.get(byDateURL, () => HttpResponse.json(datedWorkout(99))),
      http.get(detailURL, () => HttpResponse.json(detail(2))),
    );
    await expect(getRecordedProgress(42, date)).rejects.toThrow("記録済みセットの内訳を確認できませんでした。");
  });

  it("セット内訳が欠けた応答を未記録とみなさない", async () => {
    server.use(http.get(byDateURL, () => HttpResponse.json({ workout_id: 42 })));
    await expect(getRecordedProgress(42, date)).rejects.toThrow("記録済みセットの内訳を確認できませんでした。");
  });

  it("集計が欠けた応答を未記録とみなさない", async () => {
    server.use(
      http.get(byDateURL, notFound),
      http.get(detailURL, () => HttpResponse.json({ id: 42 })),
    );
    await expect(getRecordedProgress(42, date)).rejects.toThrow("記録済みセットの内訳を確認できませんでした。");
  });

  it("日付指定APIの障害時に開始を許可しない", async () => {
    server.use(http.get(byDateURL, () => HttpResponse.json({ status: 500 }, { status: 500 })));
    await expect(getRecordedProgress(42, date)).rejects.toBeInstanceOf(ApiError);
  });
});
