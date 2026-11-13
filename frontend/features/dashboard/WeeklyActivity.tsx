import React from "react";
import { BarChart3, Loader2 } from "lucide-react";

export interface WeeklyActivityData {
  chart_data?: unknown;
  chart_dates?: unknown;
  this_week?: unknown;
}

const WEEKDAY_LABELS = ["日", "月", "火", "水", "木", "金", "土"];
const DAYS = 7;

function isCount(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0;
}

function normalizeCounts(value: unknown): number[] {
  if (Array.isArray(value) && value.length === DAYS && value.every(isCount)) return value;
  return Array(DAYS).fill(0);
}

function normalizeDates(value: unknown): (string | null)[] {
  if (Array.isArray(value) && value.length === DAYS && value.every(item => typeof item === "string" && /^\d{4}-\d{2}-\d{2}$/.test(item))) {
    return value;
  }
  return Array(DAYS).fill(null);
}

function normalizeWeek(value: unknown): { workouts: number; planned: number } | null {
  if (!value || typeof value !== "object") return null;
  const week = value as Record<string, unknown>;
  if (!isCount(week.workout_count) || !isCount(week.planned_count)) return null;
  return { workouts: week.workout_count, planned: week.planned_count };
}

// 日付はサーバー（アプリ設定のタイムゾーン）の暦日なので、ブラウザのタイムゾーンで再解釈しない。
function describeDate(date: string | null, isToday: boolean) {
  if (!date) return { short: isToday ? "今日" : "", long: isToday ? "今日" : "" };
  const [, month, day] = date.split("-").map(Number);
  const weekday = WEEKDAY_LABELS[new Date(`${date}T00:00:00Z`).getUTCDay()];
  return { short: isToday ? "今日" : weekday, long: `${month}月${day}日（${weekday}）` };
}

export function WeeklyActivity({ data }: { data: WeeklyActivityData | null }) {
  const counts = normalizeCounts(data?.chart_data);
  const dates = normalizeDates(data?.chart_dates);
  const week = normalizeWeek(data?.this_week);
  const max = Math.max(...counts);
  const total = counts.reduce((sum, count) => sum + count, 0);

  return (
    <section aria-labelledby="weekly-activity-heading" className="glass rounded-[32px] p-6 md:p-10 border border-white/5 mb-8">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-6">
        <h2 id="weekly-activity-heading" className="text-xl font-bold flex items-center gap-2">
          <BarChart3 className="w-5 h-5 text-primary" /> 直近7日の記録量
        </h2>
        {week && (
          <p className="text-sm font-bold text-white/80" data-testid="weekly-progress">
            今週 <span className="text-2xl font-black text-white">{week.workouts}</span> 回
            <span className="text-white/40"> / </span>
            予定 <span className="text-white">{week.planned}</span> 回
          </p>
        )}
      </div>

      {!data ? (
        <div className="py-6 text-center text-white/30 text-sm flex items-center justify-center gap-2">
          <Loader2 className="w-4 h-4 animate-spin" /> 記録量を読み込み中...
        </div>
      ) : (
        <>
          <ol aria-label="直近7日のセット数" className="grid grid-cols-7 gap-2 border-b border-white/10">
            {counts.map((count, index) => {
              const isToday = index === DAYS - 1;
              const label = describeDate(dates[index], isToday);
              const height = max > 0 ? Math.max((count / max) * 100, count > 0 ? 6 : 0) : 0;
              return (
                <li
                  key={dates[index] ?? index}
                  tabIndex={0}
                  aria-label={`${label.long || label.short} ${count}セット`}
                  className="group relative flex h-28 items-end justify-center rounded-md outline-none focus-visible:ring-2 focus-visible:ring-primary/60"
                >
                  <span
                    aria-hidden="true"
                    className="pointer-events-none absolute -top-1 left-1/2 z-10 -translate-x-1/2 -translate-y-full whitespace-nowrap rounded-lg bg-black/90 px-2 py-1 text-[11px] font-bold text-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
                  >
                    {label.long ? `${label.long} ` : ""}{count}セット
                  </span>
                  {count > 0 ? (
                    <div
                      aria-hidden="true"
                      data-testid="weekly-bar"
                      className="w-full max-w-7 rounded-t-[4px] bg-primary transition-[height]"
                      style={{ height: `${height}%` }}
                    />
                  ) : (
                    <div aria-hidden="true" data-testid="weekly-bar" className="h-0.5 w-full max-w-7 rounded-t-[4px] bg-white/15" />
                  )}
                </li>
              );
            })}
          </ol>
          <div aria-hidden="true" className="grid grid-cols-7 gap-2 mt-2 text-center text-[11px] font-bold">
            {counts.map((_, index) => {
              const isToday = index === DAYS - 1;
              return (
                <span key={dates[index] ?? index} className={isToday ? "text-primary" : "text-white/40"}>
                  {describeDate(dates[index], isToday).short}
                </span>
              );
            })}
          </div>
          <p className="mt-4 text-xs text-white/50">
            {total > 0
              ? `7日間で合計 ${total} セット記録しました。`
              : "直近7日の記録はまだありません。セットを保存するとここに表示されます。"}
          </p>
        </>
      )}
    </section>
  );
}
