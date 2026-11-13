import React from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { NumericInput, normalizeNumericInput, parseNumericInput } from "./NumericInput";

function Harness({ kind, onEnter, error }: { kind: "decimal" | "integer"; onEnter?: () => void; error?: string }) {
  const [value, setValue] = React.useState("0");
  const id = React.useId();
  return (
    <div>
      <label htmlFor={id}>値</label>
      <NumericInput id={id} kind={kind} value={value} onValueChange={setValue} onEnter={onEnter} error={error} />
    </div>
  );
}

it("重量はdecimal、回数はnumericのキーパッドを指定する", () => {
  render(<><Harness kind="decimal" /><Harness kind="integer" /></>);
  const [weight, reps] = screen.getAllByRole("textbox");
  expect(weight).toHaveAttribute("inputmode", "decimal");
  expect(weight).toHaveAttribute("type", "text");
  expect(reps).toHaveAttribute("inputmode", "numeric");
});

it("欄を空にでき、0に書き戻さない", async () => {
  render(<Harness kind="decimal" />);
  const input = screen.getByRole("textbox");
  await userEvent.clear(input);
  expect(input).toHaveValue("");
  await userEvent.type(input, "60");
  expect(input).toHaveValue("60");
});

it("小数欄は小数点を1つだけ受け付け、整数欄は数字だけを受け付ける", async () => {
  render(<><Harness kind="decimal" /><Harness kind="integer" /></>);
  const [weight, reps] = screen.getAllByRole("textbox");
  await userEvent.clear(weight);
  await userEvent.type(weight, "62.5.");
  expect(weight).toHaveValue("62.5");
  await userEvent.clear(reps);
  await userEvent.type(reps, "1.0e");
  expect(reps).toHaveValue("10");
});

it("Enterで保存処理を呼び、日本語入力の確定Enterでは呼ばない", async () => {
  const onEnter = vi.fn();
  render(<Harness kind="integer" onEnter={onEnter} />);
  const input = screen.getByRole("textbox");
  expect(input).toHaveAttribute("enterkeyhint", "done");
  await userEvent.type(input, "{Enter}");
  expect(onEnter).toHaveBeenCalledTimes(1);
  fireEvent.keyDown(input, { key: "Enter", isComposing: true });
  expect(onEnter).toHaveBeenCalledTimes(1);
});

it("エラーを欄に関連付けて表示する", () => {
  render(<Harness kind="integer" error="回数を入力してください。" />);
  const input = screen.getByRole("textbox", { name: "値" });
  expect(input).toHaveAttribute("aria-invalid", "true");
  expect(input).toHaveAccessibleDescription("回数を入力してください。");
});

it("全角数字と読点を半角に正規化し、空や小数点だけの値は数値にしない", () => {
  expect(normalizeNumericInput("１２．５", "decimal")).toBe("12.5");
  expect(normalizeNumericInput("12,5", "decimal")).toBe("12.5");
  expect(normalizeNumericInput("-1", "decimal")).toBeNull();
  expect(normalizeNumericInput("1.5", "integer")).toBeNull();
  expect(parseNumericInput("", "integer")).toBeNull();
  expect(parseNumericInput(".", "decimal")).toBeNull();
  expect(parseNumericInput("12.", "decimal")).toBe(12);
  expect(parseNumericInput("0", "decimal")).toBe(0);
});
