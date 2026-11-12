import React from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { Modal } from "./Modal";
import { expectAccessible } from "@/test/a11y";

function Example() {
  const [open, setOpen] = React.useState(false);
  const [nested, setNested] = React.useState(false);
  return <>
    <button onClick={() => setOpen(true)}>設定を開く</button>
    {open && <Modal label="設定" onClose={() => setOpen(false)} className="">
      <button onClick={() => setOpen(false)}>閉じる</button>
      <button onClick={() => setNested(true)}>種目を選ぶ</button>
      {nested && <Modal label="種目" onClose={() => setNested(false)} className="">
        <button onClick={() => setNested(false)}>戻る</button>
      </Modal>}
    </Modal>}
  </>;
}

it("名前付きdialogへフォーカスし、Tabを循環させ、Escapeで開いたボタンへ戻す", async () => {
  const user = userEvent.setup();
  const { container } = render(<Example />);
  const trigger = screen.getByRole("button", { name: "設定を開く" });
  await user.click(trigger);
  const dialog = screen.getByRole("dialog", { name: "設定" });
  const first = within(dialog).getByRole("button", { name: "閉じる" });
  const last = within(dialog).getByRole("button", { name: "種目を選ぶ" });
  expect(dialog).toHaveAttribute("aria-modal", "true");
  expect(container.inert).toBe(true);
  expect(first).toHaveFocus();
  await user.tab({ shift: true });
  expect(last).toHaveFocus();
  await user.tab();
  expect(first).toHaveFocus();
  trigger.focus();
  expect(first).toHaveFocus();
  await expectAccessible(dialog);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(container.inert).toBeFalsy();
  expect(trigger).toHaveFocus();
});

it("重ねたモーダルのEscapeは手前だけを閉じ、スクロールロックとフォーカスを戻す", async () => {
  const user = userEvent.setup();
  document.body.style.overflow = "auto";
  document.body.style.touchAction = "pan-y";
  render(<Example />);
  await user.click(screen.getByRole("button", { name: "設定を開く" }));
  const parent = screen.getByRole("dialog", { name: "設定" });
  const trigger = within(parent).getByRole("button", { name: "種目を選ぶ" });
  await user.click(trigger);
  expect(within(screen.getByRole("dialog", { name: "種目" })).getByRole("button", { name: "戻る" })).toHaveFocus();
  expect(parent.parentElement?.inert).toBe(true);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog", { name: "種目" })).not.toBeInTheDocument();
  expect(parent).toBeInTheDocument();
  expect(trigger).toHaveFocus();
  expect(document.body.style.overflow).toBe("hidden");
  await user.keyboard("{Escape}");
  expect(document.body.style.overflow).toBe("auto");
  expect(document.body.style.touchAction).toBe("pan-y");
  document.body.style.overflow = "";
  document.body.style.touchAction = "";
});

it("内部クリックでは閉じず、背景クリックで閉じる。再描画後も最新のコールバックを使う", async () => {
  const onClose = vi.fn();
  const latest = vi.fn();
  const user = userEvent.setup();
  const view = render(<Modal label="確認" onClose={onClose} className=""><button>操作</button></Modal>);
  await user.click(screen.getByRole("button", { name: "操作" }));
  expect(onClose).not.toHaveBeenCalled();
  view.rerender(<Modal label="確認" onClose={latest} className=""><button>操作</button></Modal>);
  expect(screen.getByRole("button", { name: "操作" })).toHaveFocus();
  await user.click(screen.getByRole("dialog").parentElement!);
  expect(latest).toHaveBeenCalledOnce();
  await user.keyboard("{Escape}");
  expect(latest).toHaveBeenCalledTimes(2);
  expect(onClose).not.toHaveBeenCalled();
});

it("操作可能な要素がなければdialog自体にフォーカスを保つ", async () => {
  const user = userEvent.setup();
  render(<Modal label="待機" onClose={vi.fn()} className=""><button disabled>保存中</button><button hidden>非表示</button></Modal>);
  const dialog = screen.getByRole("dialog");
  expect(dialog).toHaveFocus();
  await user.tab();
  expect(dialog).toHaveFocus();
  await user.tab({ shift: true });
  expect(dialog).toHaveFocus();
});
