import React from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import ErrorPage from "./error";

it("予期しないエラーでは再試行で画面をリセットできる", async () => {
  const reset = vi.fn();
  render(<ErrorPage error={new Error("internal details")} reset={reset} />);
  const error = screen.getByRole("alert");
  expect(error).toHaveTextContent("画面の表示に失敗しました。");
  expect(error).not.toHaveTextContent("internal details");
  await userEvent.click(within(error).getByRole("button", { name: "再試行" }));
  expect(reset).toHaveBeenCalledOnce();
});
