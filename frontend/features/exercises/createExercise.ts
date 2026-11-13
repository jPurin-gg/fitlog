import { ApiError, apiFetch } from "@/lib/api";
import { DEFAULT_CATEGORY } from "./exerciseOptions";
import type { Exercise } from "./types";

// 種目追加フォームの入力。どの画面から追加しても、この形で作成APIへ送る。
export interface ExerciseDraft {
  name: string;
  primaryMuscle: string;
  category: string;
  equipment: string;
  level: string;
  secondaryMuscles: string;
  instructions: string;
}

export function emptyExerciseDraft(name = ""): ExerciseDraft {
  return {
    name,
    primaryMuscle: "",
    category: DEFAULT_CATEGORY,
    equipment: "その他",
    level: "初級",
    secondaryMuscles: "",
    instructions: "",
  };
}

export function toCreateExerciseRequest(draft: ExerciseDraft) {
  return {
    name: draft.name.trim(),
    category: draft.category,
    equipment: draft.equipment,
    level: draft.level,
    primary_muscles: draft.primaryMuscle ? [draft.primaryMuscle] : [],
    secondary_muscles: draft.secondaryMuscles.split(/[,、，]/).map(value => value.trim()).filter(Boolean),
    instructions: draft.instructions.split("\n").map(value => value.trim()).filter(Boolean),
  };
}

export type CreateExerciseResult =
  | { status: "created"; exercise: Exercise }
  | { status: "duplicate"; existing: Exercise; message: string };

// 種目を作成する。同名（正規化後）の種目があればAPIは409と既存種目を返すので、作成せず既存を返す。
export async function createExercise(draft: ExerciseDraft): Promise<CreateExerciseResult> {
  try {
    const exercise = await apiFetch<Exercise>("/api/exercises", {
      method: "POST",
      body: JSON.stringify(toCreateExerciseRequest(draft)),
    });
    return { status: "created", exercise };
  } catch (error) {
    if (error instanceof ApiError && error.status === 409 && isExercise(error.problem?.existing)) {
      return { status: "duplicate", existing: error.problem.existing, message: error.message };
    }
    throw error;
  }
}

function isExercise(value: unknown): value is Exercise {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Partial<Exercise>;
  return typeof candidate.id === "string" && typeof candidate.name === "string";
}
