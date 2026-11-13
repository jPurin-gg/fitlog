import React from "react";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/font/google", () => ({
  Geist: () => ({ variable: "font-geist-sans" }),
  Geist_Mono: () => ({ variable: "font-geist-mono" }),
}));
vi.mock("./globals.css", () => ({}));
vi.mock("next/navigation", () => ({ usePathname: () => "/" }));

import RootLayout from "./layout";
import { BottomNav } from "@/components/BottomNav";

const frontendRoot = resolve(__dirname, "..");

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap(name => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sourceFiles(path);
    return /\.tsx$/.test(name) && !/\.test\.tsx$/.test(name) ? [path] : [];
  });
}

describe("画面下部の余白", () => {
  it("body は下パディングを持たず、高さ100dvhの縦 flex でページを並べる", () => {
    const html = RootLayout({ children: <div /> }) as React.ReactElement<{ children: React.ReactElement<{ className: string }> }>;
    const classes = html.props.children.props.className.split(/\s+/);
    expect(classes.some(name => /^pb-/.test(name))).toBe(false);
    expect(classes).toEqual(expect.arrayContaining(["flex", "flex-col", "min-h-dvh"]));
  });

  it("下部ナビ用の余白は BottomNav のスペーサー1つだけで持つ", () => {
    const { container } = render(<BottomNav />);
    const spacer = container.firstElementChild;
    expect(spacer?.className).toContain("h-20");
    expect(spacer?.className).toContain("md:h-24");
  });

  it("ページのラッパーは min-h-screen を使わず、スペーサーと合わせて1画面に収まる", () => {
    const offenders = ["app", "components", "features"]
      .flatMap(dir => sourceFiles(join(frontendRoot, dir)))
      .filter(path => readFileSync(path, "utf8").includes("min-h-screen"));
    expect(offenders).toEqual([]);
  });
});
