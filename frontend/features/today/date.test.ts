import { afterEach, describe, expect, it, vi } from "vitest";
import { formatDateKey, formatDateTime, isFutureDateKey, monthKeyOf, toDateKey } from "./date";

afterEach(() => { vi.useRealTimers(); });

describe("サーバーの日付キー", () => {
  it("過去・今日・未来をブラウザの時計に関係なく判定する", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    // ブラウザ側はまだ前日でも、サーバーの今日（2026-08-25）を基準にする。
    vi.setSystemTime(new Date("2026-08-24T16:30:00Z"));
    expect(isFutureDateKey("2026-08-24", "2026-08-25")).toBe(false);
    expect(isFutureDateKey("2026-08-25", "2026-08-25")).toBe(false);
    expect(isFutureDateKey("2026-08-26", "2026-08-25")).toBe(true);
    expect(isFutureDateKey("2026-09-01", "2026-08-31")).toBe(true);
    expect(isFutureDateKey("2027-01-01", "2026-12-31")).toBe(true);
  });

  it("年月日からキーを作り、月を取り出す", () => {
    expect(toDateKey(2026, 0, 5)).toBe("2026-01-05");
    expect(toDateKey(2026, 11, 31)).toBe("2026-12-31");
    expect(monthKeyOf("2026-12-31")).toBe("2026-12");
  });

  it("日付キーの曜日はタイムゾーンに左右されない", () => {
    expect(formatDateKey("2026-08-25", { month: "long", day: "numeric", weekday: "long" })).toBe("8月25日火曜日");
    expect(formatDateKey("2026-03-01", { month: "long", day: "numeric", weekday: "long" })).toBe("3月1日日曜日");
  });
});

describe("アプリのタイムゾーンでの時刻表示", () => {
  it("指定したタイムゾーンの日付と時刻で整形する", () => {
    expect(formatDateTime("2026-08-24T16:30:00Z", "Asia/Tokyo")).toMatch(/8月25日.*01:30/);
    expect(formatDateTime("2026-08-24T16:30:00Z", "America/Los_Angeles")).toMatch(/8月24日.*09:30/);
  });

  it("不正なタイムゾーンや日時でも例外にしない", () => {
    expect(formatDateTime("2026-08-24T16:30:00Z", "Invalid/Zone")).toMatch(/8月/);
    expect(formatDateTime("not-a-date", "Asia/Tokyo")).toBe("not-a-date");
  });
});
