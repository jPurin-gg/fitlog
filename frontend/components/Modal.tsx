"use client";

import React from "react";
import { createPortal } from "react-dom";
import { motion } from "framer-motion";

interface ModalProps {
  label: string;
  onClose: () => void;
  children: React.ReactNode;
  className: string;
}

const layers: HTMLElement[] = [];
const backgroundState = new Map<HTMLElement, boolean>();
let previousOverflow = "";

function updateBackground() {
  const top = layers.at(-1);
  for (const child of document.body.children) {
    if (!(child instanceof HTMLElement)) continue;
    if (!backgroundState.has(child)) backgroundState.set(child, child.inert);
    child.inert = top ? child !== top : backgroundState.get(child) ?? false;
  }
  if (!top) {
    for (const [element, inert] of backgroundState) element.inert = inert;
    backgroundState.clear();
  }
}

function focusableElements(panel: HTMLElement) {
  return Array.from(panel.querySelectorAll<HTMLElement>(
    'button, a[href], input, select, textarea, [tabindex]',
  )).filter(element => element.tabIndex >= 0
    && !element.matches(":disabled")
    && !element.closest('[hidden], [inert], [aria-hidden="true"]')
    && getComputedStyle(element).display !== "none"
    && getComputedStyle(element).visibility !== "hidden");
}

export function Modal({ label, onClose, children, className }: ModalProps) {
  const layerRef = React.useRef<HTMLDivElement>(null);
  const panelRef = React.useRef<HTMLDivElement>(null);
  const onCloseRef = React.useRef(onClose);
  React.useLayoutEffect(() => { onCloseRef.current = onClose; });

  React.useLayoutEffect(() => {
    const layer = layerRef.current!;
    const panel = panelRef.current!;
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (layers.length === 0) {
      previousOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
    }
    layers.push(layer);
    updateBackground();
    const focusFirst = () => (focusableElements(panel)[0] ?? panel).focus();
    focusFirst();

    const onKeyDown = (event: KeyboardEvent) => {
      if (layers.at(-1) !== layer) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        onCloseRef.current();
      } else if (event.key === "Tab") {
        const elements = focusableElements(panel);
        const first = elements[0] ?? panel;
        const last = elements.at(-1) ?? panel;
        if (event.shiftKey && (document.activeElement === first || document.activeElement === panel)
          || !event.shiftKey && (document.activeElement === last || document.activeElement === panel)
          || !panel.contains(document.activeElement)) {
          event.preventDefault();
          (event.shiftKey ? last : first).focus();
        }
      }
    };
    const onFocusIn = (event: FocusEvent) => {
      if (layers.at(-1) === layer && !panel.contains(event.target as Node)) focusFirst();
    };
    document.addEventListener("keydown", onKeyDown, true);
    document.addEventListener("focusin", onFocusIn);
    return () => {
      document.removeEventListener("keydown", onKeyDown, true);
      document.removeEventListener("focusin", onFocusIn);
      layers.splice(layers.indexOf(layer), 1);
      updateBackground();
      if (layers.length === 0) {
        document.body.style.overflow = previousOverflow;
      }
      if (trigger?.isConnected && !trigger.closest("[inert]")) trigger.focus();
    };
  }, []);

  if (typeof document === "undefined") return null;

  return createPortal(
    <div
      ref={layerRef}
      className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/80 backdrop-blur-md overflow-hidden overscroll-none text-white"
      onClick={event => {
        event.stopPropagation();
        if (event.target === event.currentTarget && layers.at(-1) === layerRef.current) onClose();
      }}
    >
      <motion.div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        initial={{ opacity: 0, y: 20, scale: 0.95 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        className={className}
      >
        {children}
      </motion.div>
    </div>,
    document.body,
  );
}
