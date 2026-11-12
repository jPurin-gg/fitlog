"use client";

import React from "react";
import { Trophy } from "lucide-react";
import type { WorkoutDetailSet } from "../types";

interface ExerciseGroup {
  exerciseID: string;
  name: string;
  sets: WorkoutDetailSet[];
}

// 記録順を保ったまま、同じ種目のセットをまとめる。
function groupByExercise(sets: WorkoutDetailSet[]): ExerciseGroup[] {
  const groups: ExerciseGroup[] = [];
  const byExercise = new Map<string, ExerciseGroup>();
  for (const set of sets) {
    let group = byExercise.get(set.exercise_id);
    if (!group) {
      group = { exerciseID: set.exercise_id, name: set.exercise_name || set.exercise_id, sets: [] };
      byExercise.set(set.exercise_id, group);
      groups.push(group);
    }
    group.sets.push(set);
  }
  return groups;
}

export function WorkoutSetList({ sets }: { sets: WorkoutDetailSet[] }) {
  const groups = groupByExercise(sets);

  return (
    <section className="mt-8">
      <h2 className="font-bold text-lg mb-4">セットごとの記録</h2>
      {groups.length > 0 ? (
        <div className="space-y-3">
          {groups.map(group => (
            <div key={group.exerciseID} className="bg-white/5 rounded-2xl border border-white/5 p-4">
              <h3 className="font-bold text-white leading-tight mb-3">{group.name}</h3>
              <ol className="space-y-2" aria-label={`${group.name}のセット`}>
                {group.sets.map((set, index) => (
                  <li key={set.id} className="bg-black/25 rounded-xl p-3">
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
                      <span className="text-[10px] font-black text-primary">{index + 1}セット目</span>
                      <span className="text-sm font-bold">{set.weight}kg × {set.reps}回</span>
                      {set.is_pr && (
                        <span className="text-[10px] font-black text-black bg-primary px-2 py-0.5 rounded-full flex items-center gap-1">
                          <Trophy className="w-3 h-3" /> 自己ベスト
                        </span>
                      )}
                    </div>
                    <p className={`text-xs mt-1 whitespace-pre-wrap ${set.feeling ? "text-white/70" : "text-white/30"}`}>
                      {set.feeling ? `感想: ${set.feeling}` : "感想なし"}
                    </p>
                  </li>
                ))}
              </ol>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-sm text-white/45 bg-white/5 rounded-2xl border border-white/5 p-5">記録されたセットはありません。</p>
      )}
    </section>
  );
}
