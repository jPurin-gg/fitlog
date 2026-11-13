import { parseNumericInput, type NumericKind } from "@/components/NumericInput";
import type { CalendarPlanExercise, CalendarWorkoutSet } from "./types";

// 編集中は空欄や打ちかけの値を表せるよう、数値欄を文字列で持つ。
export type WorkoutSetDraft = Omit<CalendarWorkoutSet, "weight" | "reps"> & { weight: string; reps: string };
export type PlanExerciseDraft = Omit<CalendarPlanExercise, "planned_sets" | "target_weight" | "target_reps"> & {
  planned_sets: string;
  target_weight: string;
  target_reps: string;
};

/** 欄ごとのエラー。キーは fieldKey(index, field) または欄名。 */
export type FieldErrors = Record<string, string>;

export const fieldKey = (index: number, field: string) => `${index}.${field}`;

export function numberToInput(value: number | null | undefined): string {
  return typeof value === "number" && Number.isFinite(value) ? String(value) : "";
}

export function toWorkoutSetDraft(set: CalendarWorkoutSet): WorkoutSetDraft {
  return { ...set, weight: numberToInput(set.weight), reps: numberToInput(set.reps), feeling: set.feeling || "" };
}

export function toPlanExerciseDraft(exercise: CalendarPlanExercise): PlanExerciseDraft {
  return {
    ...exercise,
    planned_sets: numberToInput(exercise.planned_sets),
    target_weight: numberToInput(exercise.target_weight),
    target_reps: numberToInput(exercise.target_reps),
  };
}

interface FieldRule {
  label: string;
  kind: NumericKind;
  min: number;
  emptyHint?: string;
}

const WEIGHT: FieldRule = { label: "重量", kind: "decimal", min: 0, emptyHint: "（自重は0）" };
const REPS: FieldRule = { label: "回数", kind: "integer", min: 1 };
const PLANNED_SETS: FieldRule = { label: "セット数", kind: "integer", min: 1 };
const DURATION: FieldRule = { label: "目安分", kind: "integer", min: 15 };

/** 空欄は既定値で埋めずエラーにする。 */
function checkField(value: string, rule: FieldRule, errors: FieldErrors, key: string): number {
  if (value.trim() === "") {
    errors[key] = `${rule.label}を入力してください${rule.emptyHint ?? ""}。`;
    return Number.NaN;
  }
  const parsed = parseNumericInput(value, rule.kind);
  if (parsed === null || parsed < rule.min) {
    errors[key] = `${rule.label}は${rule.min}以上の${rule.kind === "integer" ? "整数" : "数値"}で入力してください。`;
    return Number.NaN;
  }
  return parsed;
}

export function validateWorkoutSets(sets: WorkoutSetDraft[]) {
  const errors: FieldErrors = {};
  const payload = sets.map((set, index) => ({
    exercise_id: set.exercise_id,
    weight: checkField(set.weight, WEIGHT, errors, fieldKey(index, "weight")),
    reps: checkField(set.reps, REPS, errors, fieldKey(index, "reps")),
    set_order: index + 1,
    feeling: set.feeling || "",
  }));
  return { payload, errors };
}

export function validatePlanDraft(duration: string, exercises: PlanExerciseDraft[]) {
  const errors: FieldErrors = {};
  const estimatedDurationMin = checkField(duration, DURATION, errors, "duration");
  const payload = exercises.map((exercise, index) => ({
    exercise_id: exercise.exercise_id,
    name: exercise.name,
    planned_sets: checkField(exercise.planned_sets, PLANNED_SETS, errors, fieldKey(index, "planned_sets")),
    target_weight: checkField(exercise.target_weight, WEIGHT, errors, fieldKey(index, "target_weight")),
    target_reps: checkField(exercise.target_reps, REPS, errors, fieldKey(index, "target_reps")),
  }));
  return { estimatedDurationMin, exercises: payload, errors };
}

export function withoutFieldError(errors: FieldErrors, key: string): FieldErrors {
  if (!(key in errors)) return errors;
  const next = { ...errors };
  delete next[key];
  return next;
}
