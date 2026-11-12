"use client";

import React from "react";
import { ListChecks } from "lucide-react";
import type { RecordedExercise } from "../recordedProgress";

export function RecordedSetsList({ exercises }: { exercises: RecordedExercise[] }) {
  if (exercises.length === 0) return null;
  return (
    <section aria-label="今日ここまでの記録" className="rounded-2xl border border-white/10 bg-black/25 p-4">
      <p className="mb-3 flex items-center gap-2 text-sm font-bold text-white/85">
        <ListChecks className="h-4 w-4 text-primary" /> 今日ここまでの記録
      </p>
      <ul className="space-y-3">
        {exercises.map(exercise => (
          <li key={exercise.exerciseID}>
            <div className="flex items-center justify-between gap-3">
              <span className="truncate text-sm font-bold text-white/90">{exercise.name}</span>
              <span className="whitespace-nowrap text-xs font-bold text-primary">{exercise.setCount}セット保存済み</span>
            </div>
            {exercise.sets.length > 0 && (
              <ul className="mt-2 flex flex-wrap gap-2">
                {exercise.sets.map((set, index) => (
                  <li key={set.id ?? `${set.set_order}-${index}`} className="rounded-lg bg-white/5 px-3 py-1.5 text-xs text-white/70">
                    {set.set_order}セット目 {set.weight}kg × {set.reps}回
                  </li>
                ))}
              </ul>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
