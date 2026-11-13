export const FREQUENCY_OPTIONS = [
  { id: "週1-2回", label: "週1〜2回" },
  { id: "週3-4回", label: "週3〜4回" },
  { id: "週5-6回", label: "週5〜6回" },
] as const;

export const DEFAULT_MOTIVATION = "健康維持";
export const DEFAULT_FREQUENCY = "週3-4回";

// 再生成しても保存済みの日次予定（workout_plans）は作り直されない。
// backend の TestGenerateMonthlyKeepsSavedDailyPlans と統合テストがこの説明どおりの動作を確認している。
export const DAILY_PLAN_NOTE =
  "カレンダーで保存した日ごとの予定（予定を編集した日や、記録を始めた日）は作り直されず、そのまま残ります。予定を保存していない日は、新しいプランのメニューになります。";

type PlanLike = {
  frequency?: string | null;
  rest_days?: number[] | null;
  recommended_days?: number[] | null;
} | null | undefined;

function frequencyForCount(count: number): string {
  if (count <= 2) return "週1-2回";
  if (count <= 4) return "週3-4回";
  return "週5-6回";
}

function toHalfWidthDigits(text: string): string {
  return text.replace(/[０-９]/g, digit => String.fromCharCode(digit.charCodeAt(0) - 0xfee0));
}

// 保存済みの頻度はAIが書いた文言（例: 「週3回」）のこともあるため、選択肢へ寄せる。
export function frequencyOptionFor(plan: PlanLike): string {
  const text = toHalfWidthDigits(plan?.frequency?.trim() || "");
  if (FREQUENCY_OPTIONS.some(option => option.id === text)) return text;
  const match = text.match(/週\s*(\d+)/);
  if (match) {
    const count = Number(match[1]);
    if (count >= 1 && count <= 7) return frequencyForCount(count);
  }
  const days = plan?.recommended_days?.length ?? 0;
  if (days >= 1 && days <= 7) return frequencyForCount(days);
  return DEFAULT_FREQUENCY;
}

export function restDaysFor(plan: PlanLike): number[] {
  const days = new Set((plan?.rest_days || []).filter(day => Number.isInteger(day) && day >= 0 && day <= 6));
  return [...days].sort((a, b) => a - b);
}
