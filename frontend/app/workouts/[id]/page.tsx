"use client";

import React from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Loader2 } from "lucide-react";
import { WorkoutSummaryView, type WorkoutSummary } from "@/components/WorkoutSummaryView";
import { useAuth } from "@/components/AuthGate";

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
  const { user } = useAuth();
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
        const res = await fetch(`${apiUrl}/api/workouts/${params.id}?user_id=${user.id}`);
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
  }, [apiUrl, params.id, user.id]);

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
          <div className="flex flex-wrap items-center gap-3 mb-2">
            <h1 className="text-3xl font-bold tracking-tight">{workout ? workout.title : "読み込み中"}</h1>
            {workout?.status === "active" && (
              <span className="text-[10px] font-black text-black bg-primary px-2.5 py-1 rounded-full">
                進行中
              </span>
            )}
          </div>
          <p className="text-white/50 text-sm">
            {workout
              ? workout.status === "active"
                ? `${formatDateTime(workout.started_at)} から継続中`
                : `${formatDateTime(workout.started_at)} から ${workout.ended_at ? formatDateTime(workout.ended_at) : ""} まで`
              : "記録を確認しています。"}
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
          <WorkoutSummaryView summary={workout.summary} />
        ) : null}
      </main>
    </div>
  );
}
