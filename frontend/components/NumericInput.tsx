"use client";

import React from "react";

export type NumericKind = "decimal" | "integer";

const FULL_WIDTH_DIGITS = /[０-９]/g;
const DECIMAL_PATTERN = /^\d*\.?\d*$/;
const INTEGER_PATTERN = /^\d*$/;

/**
 * 入力中の文字列を正規化する。全角数字と小数点の揺れを半角に寄せ、
 * 数値として打ちかけの形（空・"12."など）は保持する。受け付けない文字を含む場合はnull。
 */
export function normalizeNumericInput(raw: string, kind: NumericKind): string | null {
  const value = raw
    .replace(FULL_WIDTH_DIGITS, digit => String.fromCharCode(digit.charCodeAt(0) - 0xfee0))
    .replace(/[．。，,]/g, ".")
    .trim();
  const pattern = kind === "decimal" ? DECIMAL_PATTERN : INTEGER_PATTERN;
  return pattern.test(value) ? value : null;
}

/** 文字列の入力値を数値に変換する。空や数値として不完全な値はnull。 */
export function parseNumericInput(value: string, kind: NumericKind): number | null {
  const normalized = normalizeNumericInput(value, kind);
  if (!normalized || normalized === ".") return null;
  const parsed = kind === "decimal" ? Number.parseFloat(normalized) : Number.parseInt(normalized, 10);
  return Number.isFinite(parsed) ? parsed : null;
}

type NativeInputProps = Omit<
  React.InputHTMLAttributes<HTMLInputElement>,
  "type" | "value" | "defaultValue" | "onChange" | "inputMode" | "pattern"
>;

export interface NumericInputProps extends NativeInputProps {
  /** 入力値。空欄や打ちかけの値を表せるよう文字列で持つ。 */
  value: string;
  onValueChange: (value: string) => void;
  /** 重量など小数を許す欄は"decimal"、回数・セット数・時間は"integer"。 */
  kind: NumericKind;
  /** キーボードのEnter（完了）で呼ぶ保存処理。 */
  onEnter?: () => void;
  /**
   * 欄ごとのエラー。指定すると欄の直後に表示し、aria-invalidとaria-describedbyで関連付ける。
   * エラー文がラベル名に混ざらないよう、ラベルは<label>で囲まずhtmlForで関連付けること。
   */
  error?: string;
}

/**
 * スマホで数字キーパッドを出す共通の数値入力欄。
 * type="number"は空欄と0の不一致で値が書き戻されるため、文字列のtext入力とinputModeで扱う。
 */
export const NumericInput = React.forwardRef<HTMLInputElement, NumericInputProps>(function NumericInput(
  { value, onValueChange, kind, onEnter, error, onKeyDown, enterKeyHint, autoComplete, ...props },
  ref,
) {
  const errorID = React.useId();
  const describedBy = [props["aria-describedby"], error ? errorID : null].filter(Boolean).join(" ") || undefined;

  return (
    <>
      <input
        {...props}
        ref={ref}
        type="text"
        inputMode={kind === "decimal" ? "decimal" : "numeric"}
        pattern={kind === "decimal" ? "[0-9]*[.]?[0-9]*" : "[0-9]*"}
        enterKeyHint={enterKeyHint ?? (onEnter ? "done" : undefined)}
        autoComplete={autoComplete ?? "off"}
        value={value}
        aria-invalid={error ? true : props["aria-invalid"]}
        aria-describedby={describedBy}
        onChange={event => {
          const next = normalizeNumericInput(event.target.value, kind);
          if (next !== null) onValueChange(next);
        }}
        onKeyDown={event => {
          onKeyDown?.(event);
          if (event.defaultPrevented || event.key !== "Enter" || !onEnter) return;
          // 日本語入力の確定Enterでは保存しない。
          if (event.nativeEvent.isComposing || event.keyCode === 229) return;
          event.preventDefault();
          onEnter();
        }}
      />
      {error && (
        <span id={errorID} role="alert" className="block text-xs font-bold text-red-300">
          {error}
        </span>
      )}
    </>
  );
});
