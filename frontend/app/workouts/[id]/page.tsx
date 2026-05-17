"use client";

import React from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, BarChart3, Clock3, ListChecks, Loader2, Trophy } from "lucide-react";

interface WorkoutSummaryExercise {
  exercise_id: string;
  name: string;
  sets: number;
  total_reps: number;
  best_weight: number;
  total_volume: number;
}

interface WorkoutSummary {
  total_sets: number;
  total_reps: number;
  total_volume: number;
  duration_min: number;
  pr_count: number;
  exercises: WorkoutSummaryExercise[];
}

interface WorkoutDetail {
  id: number;
  title: string;
  started_at: string;
  ended_at: string;
  status: string;
  summary: WorkoutSummary;
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("ja-JP", {
    month: "long",
    day: "numeric",
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function WorkoutDetailPage() {
  const params = useParams<{ id: string }>();
  const [workout, setWorkout] = React.useState<WorkoutDetail | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

  React.useEffect(() => {
    const loadWorkout = async () => {
      setLoading(true);
      setError("");
      try {
        const res = await fetch(`${apiUrl}/api/workouts/${params.id}?user_id=1`);
        if (!res.ok) {
          setError(await res.text() || "ワークアウト履歴を読み込めませんでした。");
          return;
        }
        setWorkout(await res.json());
      } catch (e) {
        console.error(e);
        setError("バックエンドに接続できません。");
      } finally {
        setLoading(false);
      }
    };
    if (params.id) loadWorkout();
  }, [apiUrl, params.id]);

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 relative overflow-hidden">
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="max-w-3xl mx-auto relative z-10">
        <header className="mb-8">
          <Link href="/" className="w-11 h-11 glass rounded-2xl hover:bg-white/10 transition-colors flex items-center justify-center mb-5">
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <p className="text-primary text-xs font-black tracking-widest mb-2">ワークアウト履歴</p>
          <h1 className="text-3xl font-bold tracking-tight mb-2">{workout ? workout.title : "読み込み中"}</h1>
          <p className="text-white/50 text-sm">
            {workout ? `${formatDateTime(workout.started_at)} から` : "記録を確認しています。"}
          </p>
        </header>

        {loading ? (
          <div className="glass rounded-[32px] p-10 text-center text-white/45">
            <Loader2 className="w-7 h-7 animate-spin mx-auto mb-3" />
            履歴を読み込み中...
          </div>
        ) : error ? (
          <div className="glass rounded-[32px] p-8 border border-red-500/20 bg-red-500/10">
            <p className="text-red-300 font-bold mb-2">エラーが発生しました</p>
            <p className="text-red-200/70 text-sm">{error}</p>
          </div>
        ) : workout ? (
          <>
            <section className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
              <SummaryMetric icon={<ListChecks className="w-5 h-5" />} label="セット数" value={`${workout.summary.total_sets}`} unit="セット" />
              <SummaryMetric icon={<BarChart3 className="w-5 h-5" />} label="総ボリューム" value={Math.round(workout.summary.total_volume).toLocaleString("ja-JP")} unit="kg" />
              <SummaryMetric icon={<Clock3 className="w-5 h-5" />} label="時間" value={`${workout.summary.duration_min}`} unit="分" />
              <SummaryMetric icon={<Trophy className="w-5 h-5" />} label="自己更新" value={`${workout.summary.pr_count}`} unit="回" />
            </section>

            <section>
              <h2 className="font-bold text-lg mb-4">種目別</h2>
              {workout.summary.exercises.length > 0 ? (
                <div className="space-y-3">
                  {workout.summary.exercises.map(ex => (
                    <div key={ex.exercise_id} className="bg-white/5 rounded-2xl border border-white/5 p-4">
                      <div className="flex items-start justify-between gap-4 mb-3">
                        <h3 className="font-bold text-white leading-tight">{ex.name}</h3>
                        <span className="text-xs font-bold text-primary bg-primary/10 border border-primary/20 rounded-full px-3 py-1 whitespace-nowrap">
                          {ex.sets}セット
                        </span>
                      </div>
                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 text-center">
                        <SummaryMini label="回数" value={`${ex.total_reps}回`} />
                        <SummaryMini label="最大重量" value={`${ex.best_weight}kg`} />
                        <SummaryMini label="量" value={`${Math.round(ex.total_volume).toLocaleString("ja-JP")}kg`} />
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-white/45 bg-white/5 rounded-2xl border border-white/5 p-5">
                  セット記録はありません。
                </p>
              )}
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}

function SummaryMetric({ icon, label, value, unit }: { icon: React.ReactNode; label: string; value: string; unit: string }) {
  return (
    <div className="glass rounded-2xl p-4 border border-white/5 min-w-0">
      <div className="text-primary mb-3">{icon}</div>
      <p className="text-[10px] font-bold text-white/35 mb-1">{label}</p>
      <div className="flex items-baseline gap-1 min-w-0">
        <span className="text-xl sm:text-2xl font-black leading-tight break-words min-w-0">{value}</span>
        <span className="text-xs text-white/40 font-bold">{unit}</span>
      </div>
    </div>
  );
}

function SummaryMini({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-black/25 rounded-xl p-3 min-w-0">
      <p className="text-[10px] text-white/35 font-bold mb-1">{label}</p>
      <p className="text-sm font-bold">{value}</p>
    </div>
  );
}
