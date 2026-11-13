import { expect, it } from "vitest";
import { frequencyOptionFor, restDaysFor } from "./monthlyPlanForm";

it("現行プランの頻度を再生成フォームの選択肢へ寄せる", () => {
  expect(frequencyOptionFor({ frequency: "週5-6回" })).toBe("週5-6回");
  expect(frequencyOptionFor({ frequency: "週2回" })).toBe("週1-2回");
  expect(frequencyOptionFor({ frequency: "週３〜４回（全身法）" })).toBe("週3-4回");
  expect(frequencyOptionFor({ frequency: "週6回" })).toBe("週5-6回");
  expect(frequencyOptionFor({ frequency: "おまかせ", recommended_days: [1] })).toBe("週1-2回");
  expect(frequencyOptionFor({ frequency: "", recommended_days: [1, 2, 3, 4, 5] })).toBe("週5-6回");
  expect(frequencyOptionFor(null)).toBe("週3-4回");
});

it("現行プランの休息日を曜日順にそろえる", () => {
  expect(restDaysFor({ rest_days: [6, 0, 3, 3, 9] })).toEqual([0, 3, 6]);
  expect(restDaysFor({ rest_days: null })).toEqual([]);
  expect(restDaysFor(undefined)).toEqual([]);
});
