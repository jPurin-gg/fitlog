import { expect, it } from "vitest";

import { clampSetCount } from "./fitlog";

it.each([
  [1, 1], [20, 20], [21, 20], [2 ** 32, 20],
  [0, 1], [-1, 1], [1.5, 1],
  [NaN, 3], [Infinity, 3], [-Infinity, 3],
])("異常なセット数 %s でも配列の長さを %s に制限する", (input, expected) => {
  expect(Array.from({ length: clampSetCount(input) })).toHaveLength(expected);
});
