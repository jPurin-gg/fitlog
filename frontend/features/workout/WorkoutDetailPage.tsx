"use client";

import React from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, Loader2, Trash2 } from "lucide-react";
import { WorkoutSetList } from "./components/WorkoutSetList";
import { WorkoutSummaryView } from "./components/WorkoutSummaryView";
import { apiErrorMessage } from "@/lib/api";
import { formatWorkoutDateTime } from "@/lib/fitlog";
import { deleteWorkout, getWorkoutDetail } from "./api";
import type { WorkoutDetail } from "./types";

export default function WorkoutDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const [workout, setWorkout] = React.useState<WorkoutDetail | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [confirmingDelete, setConfirmingDelete] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);
  const [deleteError, setDeleteError] = React.useState("");

  React.useEffect(() => {
    const loadWorkout = async () => {
      setLoading(true);
      setError("");
      try {
        setWorkout(await getWorkoutDetail(params.id));
      } catch (e) {
        console.error(e);
        setError(apiErrorMessage(e, "ワークアウト履歴を読み込めませんでした。"));
      } finally {
        setLoading(false);
      }
    };
    if (params.id) loadWorkout();
  }, [params.id]);

  const closeDeleteDialog = () => {
    if (deleting) return;
    setConfirmingDelete(false);
    setDeleteError("");
  };

  const confirmDelete = async () => {
    if (!workout || deleting) return;
    setDeleting(true);
    setDeleteError("");
    try {
      await deleteWorkout(workout.id);
      router.replace("/workouts");
    } catch (e) {
      console.error(e);
      setDeleteError(apiErrorMessage(e, "記録を削除できませんでした。"));
      setDeleting(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 relative overflow-hidden">
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="max-w-3xl mx-auto relative z-10">
        <header className="mb-8">
          <div className="flex items-center justify-between gap-3 mb-5">
            <Link href="/" className="w-11 h-11 glass rounded-2xl hover:bg-white/10 transition-colors flex items-center justify-center" aria-label="ホームへ戻る">
              <ArrowLeft className="w-5 h-5" />
            </Link>
            <Link href="/workouts" className="text-sm font-bold text-primary hover:text-white transition-colors">
              履歴一覧
            </Link>
          </div>
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
                ? `${formatWorkoutDateTime(workout.started_at)} から継続中`
                : `${formatWorkoutDateTime(workout.started_at)} から ${workout.ended_at ? formatWorkoutDateTime(workout.ended_at) : ""} まで`
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
          <>
            <WorkoutSummaryView summary={workout.summary} />
            <WorkoutSetList sets={workout.sets ?? []} />
            <section className="mt-10 pt-6 border-t border-white/10">
              <button
                type="button"
                onClick={() => setConfirmingDelete(true)}
                className="px-4 py-3 rounded-2xl border border-red-500/30 bg-red-500/10 text-sm font-bold text-red-200 hover:bg-red-500/20 transition-colors flex items-center gap-2"
              >
                <Trash2 className="w-4 h-4" /> この記録を削除
              </button>
            </section>
          </>
        ) : null}
      </main>

      {confirmingDelete && workout && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/70 backdrop-blur-md" onClick={closeDeleteDialog}>
          <div
            role="dialog"
            aria-modal="true"
            aria-label="記録の削除"
            className="w-full max-w-md rounded-[28px] border border-white/10 bg-[#111] p-6 shadow-2xl"
            onClick={e => e.stopPropagation()}
          >
            <h2 className="text-xl font-black tracking-tight mb-3">この記録を削除しますか？</h2>
            <p className="text-sm text-white/60 leading-relaxed mb-2">
              {formatWorkoutDateTime(workout.started_at)} の「{workout.title}」を削除します。
            </p>
            <p className="text-sm text-white/60 leading-relaxed mb-5">
              セットごとの記録とAI総評も消え、元に戻せません。種目の最大重量などは残りの記録から計算し直します。
            </p>
            {deleteError && (
              <div role="alert" className="rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200 mb-4">
                {deleteError}
              </div>
            )}
            <div className="flex flex-col sm:flex-row gap-3">
              <button
                type="button"
                onClick={closeDeleteDialog}
                disabled={deleting}
                className="sm:flex-1 py-3 rounded-2xl bg-white/5 border border-white/10 font-bold text-white/70 hover:text-white hover:bg-white/10 transition-colors disabled:opacity-40"
              >
                キャンセル
              </button>
              <button
                type="button"
                onClick={confirmDelete}
                disabled={deleting}
                className="sm:flex-1 py-3 rounded-2xl bg-red-500 text-white font-black hover:bg-red-400 transition-colors disabled:opacity-40 disabled:pointer-events-none flex items-center justify-center gap-2"
              >
                {deleting ? <Loader2 className="w-5 h-5 animate-spin" /> : <Trash2 className="w-5 h-5" />}
                削除する
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
