// バックエンドの workout.MaxReps / workout.MaxWeight（#83）と同じ値に保つ。
export const MAX_REPS = 1000;
export const MAX_WEIGHT_KG = 2000;

// 過去の最大重量に対して「1.5倍を超え、かつ10kg以上重い」重量は保存前に確認する。
// 80→800kg のような桁の打ち間違いを止めつつ、通常の自己ベスト更新（数%〜数十%）では確認を出さない。
export const LARGE_WEIGHT_RATIO = 1.5;
export const LARGE_WEIGHT_MIN_INCREASE_KG = 10;

export interface SetValues {
  weight: number;
  reps: number;
}

export type SetValuesResult = { ok: true; values: SetValues } | { ok: false; message: string };

export function validateSetValues(weight: number, reps: number): string | null {
  if (!Number.isFinite(weight) || weight < 0 || weight > MAX_WEIGHT_KG) {
    return `重量は0〜${MAX_WEIGHT_KG.toLocaleString("ja-JP")}kgの範囲で入力してください。`;
  }
  if (!Number.isFinite(reps) || !Number.isInteger(reps)) {
    return "回数は小数ではなく整数で入力してください。";
  }
  if (reps < 1 || reps > MAX_REPS) {
    return `回数は1〜${MAX_REPS.toLocaleString("ja-JP")}回の整数で入力してください。`;
  }
  return null;
}

/** 入力欄の文字列を保存する数値に変換する。小数の回数は切り捨てずエラーにする。 */
export function parseSetValues(weightText: string, repsText: string): SetValuesResult {
  const weight = weightText.trim() === "" ? Number.NaN : Number(weightText);
  const reps = repsText.trim() === "" ? Number.NaN : Number(repsText);
  const message = validateSetValues(weight, reps);
  return message ? { ok: false, message } : { ok: true, values: { weight, reps } };
}

export function needsLargeWeightConfirmation(weight: number, previousMax: number): boolean {
  if (!Number.isFinite(previousMax) || previousMax <= 0) return false;
  return weight > previousMax * LARGE_WEIGHT_RATIO && weight - previousMax >= LARGE_WEIGHT_MIN_INCREASE_KG;
}
