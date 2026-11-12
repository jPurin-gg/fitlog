export function formatLocalDate(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

export function formatPlanMonth(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

// APIが返すRFC 3339の日時を、ブラウザのタイムゾーンで「8月24日(月) 19:30」のように表示する。
// 年をまたいで並ぶ一覧では withYear を指定する。
export function formatWorkoutDateTime(value?: string, options: { withYear?: boolean } = {}): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const format: Intl.DateTimeFormatOptions = {
    month: "long",
    day: "numeric",
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  };
  if (options.withYear) format.year = "numeric";
  return date.toLocaleString("ja-JP", format);
}

export function displayPlanText(text?: string): string {
  if (!text) return "";
  return text
    .replace("PPL法 (Push/Pull/Legs)", "PPL法（押す・引く・脚）")
    .replace("Full Body", "全身")
    .replace("Push (胸・肩・三頭)", "押す日（胸・肩・三頭）")
    .replace("Pull (背中・二頭)", "引く日（背中・二頭）")
    .replace("Legs (脚・腹)", "脚の日（脚・腹）")
    .replace("全身 A", "全身その1")
    .replace("全身 B", "全身その2")
    .replace(/^Day ([1-7])$/, "$1日目")
    .replace(/^Push$/, "押す日")
    .replace(/^Pull$/, "引く日")
    .replace(/^Legs$/, "脚の日")
    .replace(/^Strength$/, "筋トレ")
    .replace(/^Workout$/, "ワークアウト");
}
