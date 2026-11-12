import React from "react";
import Link from "next/link";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LEAVE_CONFIRMATION_MESSAGE, useLeaveConfirmation } from "./useLeaveConfirmation";

function Navigation({ active, onNavigate }: { active: boolean; onNavigate: () => void }) {
  useLeaveConfirmation(active);
  return <Link href="/calendar" onClick={event => { event.preventDefault(); onNavigate(); }}>カレンダー</Link>;
}

describe("useLeaveConfirmation", () => {
  it("未保存入力のあるリンク移動は取消で止め、承認したときだけ通す", () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    const navigate = vi.fn();
    render(<Navigation active onNavigate={navigate} />);

    fireEvent.click(screen.getByRole("link", { name: "カレンダー" }));
    expect(confirm).toHaveBeenCalledWith(LEAVE_CONFIRMATION_MESSAGE);
    expect(navigate).not.toHaveBeenCalled();

    confirm.mockReturnValue(true);
    fireEvent.click(screen.getByRole("link", { name: "カレンダー" }));
    expect(navigate).toHaveBeenCalledTimes(1);
  });

  it("保存後とアンマウント後はリンク・再読み込みの確認を解除する", () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    const navigate = vi.fn();
    const view = render(<Navigation active onNavigate={navigate} />);
    const unload = () => window.dispatchEvent(new Event("beforeunload", { cancelable: true }));
    expect(unload()).toBe(false);

    view.rerender(<Navigation active={false} onNavigate={navigate} />);
    fireEvent.click(screen.getByRole("link", { name: "カレンダー" }));
    expect(navigate).toHaveBeenCalledTimes(1);
    expect(confirm).not.toHaveBeenCalled();
    expect(unload()).toBe(true);

    view.rerender(<Navigation active onNavigate={navigate} />);
    view.unmount();
    expect(unload()).toBe(true);
  });

  it("新しいタブを開く操作では元の入力が残るので確認しない", () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<Navigation active onNavigate={vi.fn()} />);
    fireEvent.click(screen.getByRole("link", { name: "カレンダー" }), { ctrlKey: true });
    expect(confirm).not.toHaveBeenCalled();
  });
});
