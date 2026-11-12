import { describe, expect, it } from "vitest";
import { addRecordedSet, emptyRecordedProgress, nextSetNumbers, progressFromSets, resumePlanIndex } from "./recordedProgress";
import type { RecordedSet } from "./types";

function savedSet(id: number, exerciseID: string, order: number): RecordedSet {
  return { id, exercise_id: exerciseID, exercise_name: exerciseID, set_order: order, weight: 60, reps: 8 };
}

describe("保存済みセットの進行状況", () => {
  it("2セットから次の番号3と全セットを復元する", () => {
    const sets = [savedSet(1, "bench", 1), savedSet(2, "bench", 2)];
    const progress = progressFromSets(sets);

    expect(progress.exercises[0]).toMatchObject({ exerciseID: "bench", setCount: 2, nextSet: 3, sets });
    expect(nextSetNumbers(progress)).toEqual({ bench: 3 });
    expect(progress.lastExerciseID).toBe("bench");
    expect(emptyRecordedProgress).toEqual({ exercises: [], lastExerciseID: null });
  });

  it("種目へ戻った場合も種目別番号と直近の種目を復元する", () => {
    const progress = progressFromSets([
      savedSet(1, "bench", 1),
      savedSet(2, "squat", 1),
      savedSet(3, "bench", 2),
    ]);

    expect(nextSetNumbers(progress)).toEqual({ bench: 3, squat: 2 });
    expect(progress.lastExerciseID).toBe("bench");
    expect(resumePlanIndex(["bench", "squat"], progress)).toBe(0);
  });

  it("欠番があっても最大番号より後を使う", () => {
    const progress = progressFromSets([savedSet(1, "bench", 1), savedSet(2, "bench", 4)]);
    expect(nextSetNumbers(progress)).toEqual({ bench: 5 });
  });

  it("過去の重複番号があっても保存件数より後を使う", () => {
    const progress = progressFromSets([savedSet(1, "bench", 1), savedSet(2, "bench", 1), savedSet(3, "bench", 2)]);
    expect(nextSetNumbers(progress)).toEqual({ bench: 4 });
    expect(progress.exercises[0].sets).toHaveLength(3);
  });

  it("同じ保存IDの再送結果を重複して数えない", () => {
    const set = savedSet(1, "bench", 1);
    const progress = progressFromSets([set]);
    expect(addRecordedSet(progress, set)).toBe(progress);
  });

  it("計画外の種目は記録済みの計画種目の次の位置で再開する", () => {
    const progress = progressFromSets([savedSet(1, "bench", 1), savedSet(2, "rowing", 1)]);
    expect(resumePlanIndex(["bench", "squat", "press"], progress)).toBe(1);
  });
});
