import axe from "axe-core";
import { expect } from "vitest";

export async function expectAccessible(element: HTMLElement) {
  // jsdomにはレイアウト・色の計算がないため、コントラストはブラウザE2Eで検査する。
  const results = await axe.run(element, { rules: { "color-contrast": { enabled: false } } });
  expect(results.violations.map(({ id, nodes }) => ({ id, targets: nodes.map(node => node.target) }))).toEqual([]);
}
