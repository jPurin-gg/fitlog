import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/font/google", () => ({
  Geist: () => ({ variable: "font-geist-sans" }),
  Geist_Mono: () => ({ variable: "font-geist-mono" }),
}));
vi.mock("./globals.css", () => ({}));

const css = readFileSync(resolve(__dirname, "globals.css"), "utf8");

function rootBlock(source: string) {
  const match = source.match(/(^|\n):root\s*\{([^}]*)\}/);
  if (!match) throw new Error(":root block not found in globals.css");
  return match[2];
}

describe("ダーク固定の配色", () => {
  it(":root で color-scheme を dark と宣言し、背景を暗色にしている", () => {
    const root = rootBlock(css);
    expect(root).toMatch(/color-scheme:\s*dark\s*;/);
    expect(root).toMatch(/--background:\s*#0a0a0a\s*;/i);
  });

  it("OS の配色設定で背景が切り替わるメディアクエリを持たない", () => {
    expect(css).not.toMatch(/prefers-color-scheme/);
  });

  it("theme-color と color-scheme を背景と同じ暗色で viewport に指定している", async () => {
    const { viewport } = await import("./layout");
    expect(viewport).toMatchObject({ themeColor: "#0a0a0a", colorScheme: "dark" });
  });
});
