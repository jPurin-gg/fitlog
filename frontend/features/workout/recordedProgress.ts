import type { RecordedSet } from "./types";

export interface RecordedExercise {
  exerciseID: string;
  name: string;
  setCount: number;
  nextSet: number;
  sets: RecordedSet[];
}

export interface RecordedProgress {
  // 最初に記録した順
  exercises: RecordedExercise[];
  // 直近に記録した種目
  lastExerciseID: string | null;
}

export const emptyRecordedProgress: RecordedProgress = { exercises: [], lastExerciseID: null };

// 次のセット番号は保存済みの件数と最大番号の両方より大きくする。
// 過去の記録で番号が重複・欠番していても、同じ set_order を再び使わない。
export function addRecordedSet(progress: RecordedProgress, set: RecordedSet): RecordedProgress {
  const current = progress.exercises.find(exercise => exercise.exerciseID === set.exercise_id);
  if (current && set.id !== undefined && current.sets.some(saved => saved.id === set.id)) {
    return progress;
  }
  const base: RecordedExercise = current ?? {
    exerciseID: set.exercise_id,
    name: set.exercise_name || set.exercise_id,
    setCount: 0,
    nextSet: 1,
    sets: [],
  };
  const setCount = base.setCount + 1;
  const updated: RecordedExercise = {
    ...base,
    setCount,
    nextSet: Math.max(base.nextSet, set.set_order + 1, setCount + 1),
    sets: [...base.sets, set],
  };
  return {
    exercises: current
      ? progress.exercises.map(exercise => (exercise === current ? updated : exercise))
      : [...progress.exercises, updated],
    lastExerciseID: set.exercise_id,
  };
}

export function progressFromSets(sets: RecordedSet[]): RecordedProgress {
  return sets.reduce(addRecordedSet, emptyRecordedProgress);
}

export function nextSetNumbers(progress: RecordedProgress): Record<string, number> {
  return Object.fromEntries(progress.exercises.map(exercise => [exercise.exerciseID, exercise.nextSet]));
}

// 直近に記録した種目が計画の何番目にあたるかを返す。記録中に計画外の種目へ変更していた場合は、
// 記録済みの計画種目の次の位置とみなす。
export function resumePlanIndex(planExerciseIDs: string[], progress: RecordedProgress): number {
  const index = progress.lastExerciseID === null ? -1 : planExerciseIDs.indexOf(progress.lastExerciseID);
  if (index >= 0) return index;
  const recorded = new Set(progress.exercises.map(exercise => exercise.exerciseID));
  let lastRecorded = -1;
  planExerciseIDs.forEach((id, position) => {
    if (recorded.has(id)) lastRecorded = position;
  });
  return Math.max(0, Math.min(lastRecorded + 1, planExerciseIDs.length - 1));
}
