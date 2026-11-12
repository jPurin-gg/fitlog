"use client";

import React from "react";
import Link from "next/link";
import { ArrowLeft, ChevronLeft, ChevronRight, Loader2 } from "lucide-react";
import { apiErrorMessage } from "@/lib/api";
import { formatWorkoutDateTime } from "@/lib/fitlog";
import { getWorkoutHistory } from "./api";
import type { WorkoutHistoryPage as WorkoutHistoryData } from "./types";

const PAGE_SIZE = 20;

export default function WorkoutHistoryPage() {
  const [page, setPage] = React.useState(0);
  const [history, setHistory] = React.useState<WorkoutHistoryData | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    getWorkoutHistory(PAGE_SIZE, page * PAGE_SIZE, controller.signal)
      .then(data => {
        if (controller.signal.aborted) return;
        setHistory(data);
        setLoading(false);
      })
      .catch(e => {
        if (controller.signal.aborted) return;
        console.error(e);
        setError(apiErrorMessage(e, "ワークアウト履歴を読み込めませんでした。"));
        setLoading(false);
      });
    return () => controller.abort();
  }, [page]);

  const workouts = history?.workouts ?? [];
  const total = history?.total ?? 0;
  const offset = history?.offset ?? 0;
  const hasPrevious = page > 0;
  const hasNext = (page + 1) * PAGE_SIZE < total;

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 relative overflow-hidden">
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="max-w-3xl mx-auto relative z-10">
        <header className="mb-8">
          <Link href="/" className="w-11 h-11 glass rounded-2xl hover:bg-white/10 transition-colors flex items-center justify-center mb-5" aria-label="ホームへ戻る">
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <h1 className="text-3xl font-bold tracking-tight mb-2">ワークアウト履歴</h1>
          <p className="text-white/50 text-sm">完了した記録を新しい順に表示します。</p>
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
        ) : workouts.length === 0 ? (
          <div className="glass rounded-[32px] p-10 text-center">
            <p className="text-white/45 text-sm mb-6">まだ完了したワークアウトはありません。</p>
            <Link href="/workout" className="inline-flex px-6 py-3 bg-primary text-black font-bold rounded-2xl">
              記録を始める
            </Link>
          </div>
        ) : (
          <>
            <ul className="space-y-3" aria-label="ワークアウト履歴の一覧">
              {workouts.map(workout => (
                <li key={workout.id}>
                  <Link
                    href={`/workouts/${workout.id}`}
                    className="block bg-white/5 rounded-2xl p-5 border border-white/5 hover:bg-white/10 transition-colors group"
                  >
                    <div className="flex justify-between items-start gap-3 mb-1">
                      <p className="text-xs font-bold text-primary">{formatWorkoutDateTime(workout.started_at, { withYear: true })}</p>
                      <ChevronRight className="w-4 h-4 text-white/25 group-hover:text-primary transition-colors flex-shrink-0 mt-0.5" />
                    </div>
                    <h2 className="font-bold truncate mb-2">{workout.title}</h2>
                    <p className="text-xs text-white/50">
                      {workout.duration_min}分 ・ {workout.total_sets}セット ・ {Math.round(workout.total_volume).toLocaleString("ja-JP")}kg
                    </p>
                  </Link>
                </li>
              ))}
            </ul>

            <nav className="mt-6 flex items-center justify-between gap-3" aria-label="履歴のページ移動">
              <button
                type="button"
                onClick={() => setPage(current => Math.max(0, current - 1))}
                disabled={!hasPrevious}
                className="px-4 py-2 rounded-xl bg-white/5 border border-white/10 text-sm font-bold text-white/70 hover:text-white hover:bg-white/10 transition-colors disabled:opacity-30 disabled:pointer-events-none flex items-center gap-1"
              >
                <ChevronLeft className="w-4 h-4" /> 前へ
              </button>
              <p className="text-xs text-white/50">
                {offset + 1}〜{offset + workouts.length}件 / 全{total}件
              </p>
              <button
                type="button"
                onClick={() => setPage(current => current + 1)}
                disabled={!hasNext}
                className="px-4 py-2 rounded-xl bg-white/5 border border-white/10 text-sm font-bold text-white/70 hover:text-white hover:bg-white/10 transition-colors disabled:opacity-30 disabled:pointer-events-none flex items-center gap-1"
              >
                次へ <ChevronRight className="w-4 h-4" />
              </button>
            </nav>
          </>
        )}
      </main>
    </div>
  );
}
