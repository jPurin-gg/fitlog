import React from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { AISettingsProvider, AIToggle, useAISettings } from "./AISettings";

beforeEach(() => window.localStorage.clear());

it("初期OFFから切り替え、同じユーザーで復元し別ユーザーには引き継がない", async () => {
  const user = userEvent.setup();
  const view = render(<AISettingsProvider userID={1}><AIToggle /></AISettingsProvider>);
  expect(screen.getByRole("switch")).not.toBeChecked();
  await user.click(screen.getByRole("switch"));
  expect(screen.getByRole("switch")).toBeChecked();
  view.rerender(<AISettingsProvider userID={2}><AIToggle /></AISettingsProvider>);
  expect(screen.getByRole("switch")).not.toBeChecked();
  view.rerender(<AISettingsProvider userID={1}><AIToggle /></AISettingsProvider>);
  expect(screen.getByRole("switch")).toBeChecked();
});

it.each(["garbage", "false", "1"])("保存値%sをONとして扱わない", value => {
  window.localStorage.setItem("fitlog:ai-enabled:1", value);
  render(<AISettingsProvider userID={1}><AIToggle /></AISettingsProvider>);
  expect(screen.getByRole("switch")).not.toBeChecked();
});

it("ストレージを使えなくても初期OFFで操作できる", async () => {
  vi.spyOn(window.localStorage, "getItem").mockImplementation(() => { throw new Error("blocked"); });
  vi.spyOn(window.localStorage, "setItem").mockImplementation(() => { throw new Error("blocked"); });
  render(<AISettingsProvider userID={1}><AIToggle /></AISettingsProvider>);
  expect(screen.getByRole("switch")).not.toBeChecked();
  await userEvent.click(screen.getByRole("switch"));
  expect(screen.getByRole("switch")).toBeChecked();
});

it("OFF操作は同じイベント内でも非同期処理から参照できる", async () => {
  const observed = vi.fn();
  function Probe() {
    const { setEnabled, isEnabled } = useAISettings();
    return <button onClick={() => { setEnabled(false); observed(isEnabled()); }}>停止</button>;
  }
  window.localStorage.setItem("fitlog:ai-enabled:1", "true");
  render(<AISettingsProvider userID={1}><AIToggle /><Probe /></AISettingsProvider>);
  await userEvent.click(screen.getByText("停止"));
  expect(observed).toHaveBeenCalledWith(false);
});
