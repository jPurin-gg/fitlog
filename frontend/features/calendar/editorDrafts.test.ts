import { expect, it } from "vitest";
import { validatePlanDraft, validateWorkoutSets } from "./editorDrafts";

it("記録の数値欄を検証し、空欄・範囲外を欄ごとのエラーにする", () => {
  const { payload, errors } = validateWorkoutSets([
    { exercise_id: "bench", weight: "0", reps: "8", set_order: 1, feeling: "" },
    { exercise_id: "bench", weight: "", reps: "0", set_order: 2, feeling: "" },
    { exercise_id: "bench", weight: ".", reps: "", set_order: 3, feeling: "" },
  ]);
  expect(payload[0]).toEqual({ exercise_id: "bench", weight: 0, reps: 8, set_order: 1, feeling: "" });
  expect(errors).toEqual({
    "1.weight": "重量を入力してください（自重は0）。",
    "1.reps": "回数は1以上の整数で入力してください。",
    "2.weight": "重量は0以上の数値で入力してください。",
    "2.reps": "回数を入力してください。",
  });
});

it("予定の目安分は15分以上で、空欄を既定値で埋めない", () => {
  const empty = validatePlanDraft("", []);
  expect(empty.errors).toEqual({ duration: "目安分を入力してください。" });
  expect(empty.estimatedDurationMin).toBeNaN();
  expect(validatePlanDraft("10", []).errors).toEqual({ duration: "目安分は15以上の整数で入力してください。" });
  const valid = validatePlanDraft("30", [{ exercise_id: "squat", name: "スクワット", planned_sets: "3", target_weight: "0", target_reps: "10" }]);
  expect(valid.errors).toEqual({});
  expect(valid.estimatedDurationMin).toBe(30);
  expect(valid.exercises).toEqual([{ exercise_id: "squat", name: "スクワット", planned_sets: 3, target_weight: 0, target_reps: 10 }]);
});
