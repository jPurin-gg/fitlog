"use client";

import React from "react";
import { BarChart3, Clock3, ListChecks, Trophy } from "lucide-react";

export interface WorkoutSummaryExercise {
  exercise_id: string;
  name: string;
  sets: number;
  total_reps: number;
  best_weight: number;
  total_volume: number;
}

export interface WorkoutSummary {
  total_sets: number;
  total_reps: number;
  total_volume: number;
  duration_min: number;
  pr_count: number;
  exercises: WorkoutSummaryExercise[];
}

interface WorkoutSummaryViewProps {
  summary: WorkoutSummary;
  emptyText?: string;
  emptyStateClassName?: string;
  exerciseSectionClassName?: string;
}

export function WorkoutSummaryView({
  summary,
  emptyText = "セット記録はありません。",
  emptyStateClassName = "text-sm text-white/45 bg-white/5 rounded-2xl border border-white/5 p-5",
  exerciseSectionClassName = "",
}: WorkoutSummaryViewProps) {
  return (
    <>
      <section className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
        <SummaryMetric icon={<ListChecks className="w-5 h-5" />} label="セット数" value={`${summary.total_sets}`} unit="セット" />
        <SummaryMetric icon={<BarChart3 className="w-5 h-5" />} label="総ボリューム" value={Math.round(summary.total_volume).toLocaleString("ja-JP")} unit="kg" />
        <SummaryMetric icon={<Clock3 className="w-5 h-5" />} label="時間" value={`${summary.duration_min}`} unit="分" />
        <SummaryMetric icon={<Trophy className="w-5 h-5" />} label="自己更新" value={`${summary.pr_count}`} unit="回" />
      </section>

      <section className={exerciseSectionClassName}>
        <h2 className="font-bold text-lg mb-4">種目別</h2>
        {summary.exercises.length > 0 ? (
          <div className="space-y-3">
            {summary.exercises.map(ex => (
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
          <p className={emptyStateClassName}>{emptyText}</p>
        )}
      </section>
    </>
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
