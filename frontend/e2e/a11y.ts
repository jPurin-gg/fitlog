import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";

export async function expectNoSeriousViolations(page: Page) {
  // 入場アニメーション中の一時的な不透明度でコントラストを判定しない。
  for (const dialog of await page.getByRole("dialog").all()) {
    await expect(dialog).toHaveCSS("opacity", "1");
  }
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations
    .filter(violation => violation.impact === "serious" || violation.impact === "critical")
    .map(({ id, nodes }) => ({ id, targets: nodes.map(node => node.target) }))).toEqual([]);
}
