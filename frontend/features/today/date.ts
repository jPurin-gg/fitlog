// 日付キー（YYYY-MM-DD）はサーバーのタイムゾーンでの暦日を表す。
// ブラウザのタイムゾーンに左右されないよう、整形はUTCの暦日として扱う。

export function toDateKey(year: number, monthIndex: number, day: number): string {
  return `${year}-${String(monthIndex + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

export function monthKeyOf(dateKey: string): string {
  return dateKey.slice(0, 7);
}

/** 同じ形式の日付キーどうしを比べ、dateKey が todayKey より後なら true。 */
export function isFutureDateKey(dateKey: string, todayKey: string): boolean {
  return dateKey > todayKey;
}

export function formatDateKey(dateKey: string, options: Intl.DateTimeFormatOptions): string {
  const [year, month, day] = dateKey.split("-").map(Number);
  if (!year || !month || !day) return dateKey;
  return new Date(Date.UTC(year, month - 1, day)).toLocaleDateString("ja-JP", { ...options, timeZone: "UTC" });
}

/** 時刻をアプリのタイムゾーンで整形する。timeZone が不明・不正ならブラウザの設定で整形する。 */
export function formatDateTime(value: string, timeZone?: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const options: Intl.DateTimeFormatOptions = {
    month: "long",
    day: "numeric",
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  };
  try {
    return date.toLocaleString("ja-JP", timeZone ? { ...options, timeZone } : options);
  } catch {
    return date.toLocaleString("ja-JP", options);
  }
}
