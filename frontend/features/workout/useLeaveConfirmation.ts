"use client";

import { useEffect } from "react";

export const LEAVE_CONFIRMATION_MESSAGE = "保存していない入力があります。このまま移動すると入力内容は失われます。移動しますか？";

// 未保存の入力がある間だけ、再読み込み・タブを閉じる操作と、アプリ内リンクでの画面移動を確認する。
// ブラウザの「戻る」は対象外。
export function useLeaveConfirmation(active: boolean) {
  useEffect(() => {
    if (!active) return;

    const confirmUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      // returnValue を見て確認を出す古いブラウザ向け。
      event.returnValue = true;
    };

    const confirmLinkNavigation = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      if (!(event.target instanceof Element)) return;
      const anchor = event.target.closest("a[href]");
      if (!(anchor instanceof HTMLAnchorElement) || anchor.target === "_blank" || anchor.hasAttribute("download")) return;
      const destination = new URL(anchor.href, window.location.href);
      // 別オリジンへの移動はページの読み込みになるため、beforeunload 側で確認する。
      if (destination.origin !== window.location.origin) return;
      if (destination.pathname === window.location.pathname && destination.search === window.location.search) return;
      if (window.confirm(LEAVE_CONFIRMATION_MESSAGE)) return;
      event.preventDefault();
      event.stopPropagation();
    };

    window.addEventListener("beforeunload", confirmUnload);
    // Next.js の Link が遷移を始める前に止められるよう、キャプチャ段階で受け取る。
    document.addEventListener("click", confirmLinkNavigation, true);
    return () => {
      window.removeEventListener("beforeunload", confirmUnload);
      document.removeEventListener("click", confirmLinkNavigation, true);
    };
  }, [active]);
}
