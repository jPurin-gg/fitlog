import { describe, expect, it } from "vitest";

import { MAX_REPS, MAX_WEIGHT_KG, needsLargeWeightConfirmation, parseSetValues, validateSetValues } from "./setLimits";

describe("set input limits", () => {
  it("バックエンド（#83）と同じ上限値を使う", () => {
    expect(MAX_REPS).toBe(1000);
    expect(MAX_WEIGHT_KG).toBe(2000);
  });

  it.each([
    ["0", "1"],
    ["2000", "1000"],
    ["62.5", "8"],
  ])("重量 %s kg × %s 回は保存できる", (weight, reps) => {
    expect(parseSetValues(weight, reps)).toEqual({ ok: true, values: { weight: Number(weight), reps: Number(reps) } });
  });

  it("小数の回数は切り捨てずにエラーにする", () => {
    expect(parseSetValues("80", "10.5")).toEqual({ ok: false, message: "回数は小数ではなく整数で入力してください。" });
  });

  it.each([
    ["-0.5", "10", /重量/],
    ["2000.5", "10", /重量は0〜2,000kg/],
    ["", "10", /重量/],
    ["80", "0", /回数は1〜1,000回/],
    ["80", "1001", /回数は1〜1,000回/],
    ["80", "", /回数/],
  ])("範囲外の重量 %s kg × %s 回はエラーにする", (weight, reps, message) => {
    const result = parseSetValues(weight, reps);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.message).toMatch(message);
  });

  it("数値の検証はカレンダー保存でも同じ規則を使う", () => {
    expect(validateSetValues(80, 10)).toBeNull();
    expect(validateSetValues(80, 10.5)).toMatch(/整数/);
    expect(validateSetValues(2001, 10)).toMatch(/重量/);
  });

  it("過去最大の1.5倍を超え、かつ10kg以上重いときだけ確認する", () => {
    expect(needsLargeWeightConfirmation(800, 80)).toBe(true);
    expect(needsLargeWeightConfirmation(121, 80)).toBe(true);
    expect(needsLargeWeightConfirmation(120, 80)).toBe(false);
    expect(needsLargeWeightConfirmation(85, 80)).toBe(false);
    expect(needsLargeWeightConfirmation(16, 6)).toBe(true);
    expect(needsLargeWeightConfirmation(15, 6)).toBe(false);
    expect(needsLargeWeightConfirmation(800, 0)).toBe(false);
  });
});
