"use client";

import React, { useId, useState } from "react";
import { AlertCircle, Loader2, Plus } from "lucide-react";
import { apiErrorMessage } from "@/lib/api";
import { createExercise, emptyExerciseDraft, type ExerciseDraft } from "../createExercise";
import { CATEGORY_OPTIONS, EQUIPMENT_OPTIONS, LEVEL_OPTIONS, MUSCLE_OPTIONS } from "../exerciseOptions";
import type { Exercise } from "../types";

interface Props {
  initialName?: string;
  submitLabel: string;
  existingActionLabel: string;
  onCreated: (exercise: Exercise) => void;
  onSelectExisting: (exercise: Exercise) => void;
  onCancel: () => void;
}

const fieldClass = "w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white";
const labelClass = "text-xs font-black text-white/40";

// 種目追加の共通フォーム。辞書ページ・種目セレクター（代替種目モーダルからも開く）で使う。
export function ExerciseCreateForm({ initialName = "", submitLabel, existingActionLabel, onCreated, onSelectExisting, onCancel }: Props) {
  const id = useId();
  const [draft, setDraft] = useState<ExerciseDraft>(() => emptyExerciseDraft(initialName));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [duplicate, setDuplicate] = useState<{ existing: Exercise; message: string } | null>(null);

  const update = (patch: Partial<ExerciseDraft>) => {
    setDraft(prev => ({ ...prev, ...patch }));
    if ("name" in patch) setDuplicate(null);
  };

  const canSubmit = Boolean(draft.name.trim() && draft.primaryMuscle) && !submitting;

  const submit = async () => {
    if (!draft.name.trim()) {
      setError("種目名を入力してください。");
      return;
    }
    if (!draft.primaryMuscle) {
      setError("メイン部位を選択してください。");
      return;
    }
    setSubmitting(true);
    setError("");
    setDuplicate(null);
    try {
      const result = await createExercise(draft);
      if (result.status === "duplicate") {
        setDuplicate({ existing: result.existing, message: result.message });
        return;
      }
      onCreated(result.exercise);
    } catch (e) {
      console.error(e);
      setError(apiErrorMessage(e, "通信エラーが発生しました。"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-5 text-white">
      <div className="space-y-2">
        <label htmlFor={`${id}-name`} className={labelClass}>種目名 <span className="text-red-400">*</span></label>
        <input
          id={`${id}-name`}
          type="text"
          value={draft.name}
          onChange={e => update({ name: e.target.value })}
          placeholder="例: インクラインダンベルプレス"
          className={fieldClass}
        />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className="space-y-2">
          <label htmlFor={`${id}-primary`} className={labelClass}>メイン部位 <span className="text-red-400">*</span></label>
          <select id={`${id}-primary`} value={draft.primaryMuscle} onChange={e => update({ primaryMuscle: e.target.value })} className={fieldClass}>
            <option value="">選択してください</option>
            {MUSCLE_OPTIONS.map(muscle => <option key={muscle} value={muscle}>{muscle}</option>)}
          </select>
        </div>
        <div className="space-y-2">
          <label htmlFor={`${id}-category`} className={labelClass}>分類</label>
          <select id={`${id}-category`} value={draft.category} onChange={e => update({ category: e.target.value })} className={fieldClass}>
            {CATEGORY_OPTIONS.map(category => <option key={category} value={category}>{category}</option>)}
          </select>
        </div>
        <div className="space-y-2">
          <label htmlFor={`${id}-equipment`} className={labelClass}>器具</label>
          <select id={`${id}-equipment`} value={draft.equipment} onChange={e => update({ equipment: e.target.value })} className={fieldClass}>
            {EQUIPMENT_OPTIONS.map(equipment => <option key={equipment} value={equipment}>{equipment}</option>)}
          </select>
        </div>
        <div className="space-y-2">
          <label htmlFor={`${id}-level`} className={labelClass}>難易度</label>
          <select id={`${id}-level`} value={draft.level} onChange={e => update({ level: e.target.value })} className={fieldClass}>
            {LEVEL_OPTIONS.map(level => <option key={level} value={level}>{level}</option>)}
          </select>
        </div>
      </div>

      <div className="space-y-2">
        <label htmlFor={`${id}-secondary`} className={labelClass}>サブ部位（任意・カンマ区切り）</label>
        <input
          id={`${id}-secondary`}
          type="text"
          value={draft.secondaryMuscles}
          onChange={e => update({ secondaryMuscles: e.target.value })}
          placeholder="例: 肩, 上腕三頭筋"
          className={fieldClass}
        />
      </div>

      <div className="space-y-2">
        <label htmlFor={`${id}-instructions`} className={labelClass}>手順メモ（任意・1行ずつ）</label>
        <textarea
          id={`${id}-instructions`}
          value={draft.instructions}
          onChange={e => update({ instructions: e.target.value })}
          placeholder={"例:\nベンチを30度くらいにする\n胸を張って肩をすくめない\n最後までフォーム優先"}
          className={`${fieldClass} min-h-[110px]`}
        />
      </div>

      {duplicate && (
        <div role="alert" className="rounded-2xl border border-primary/30 bg-primary/10 p-4 text-sm">
          <p className="text-white/90">{duplicate.message}</p>
          <button
            type="button"
            onClick={() => onSelectExisting(duplicate.existing)}
            className="mt-3 w-full py-3 bg-primary text-black font-black rounded-xl hover:scale-[1.02] active:scale-[0.98] transition-all"
          >
            {existingActionLabel}: {duplicate.existing.name}
          </button>
        </div>
      )}

      {error && (
        <div role="alert" className="rounded-2xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200 flex items-start gap-2">
          <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 pt-2">
        <button
          type="button"
          onClick={onCancel}
          className="py-3 bg-white/10 text-white font-bold rounded-xl hover:bg-white/15 transition-all"
        >
          キャンセル
        </button>
        <button
          type="button"
          onClick={submit}
          disabled={!canSubmit}
          className="py-3 bg-primary text-black font-black rounded-xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-40 disabled:pointer-events-none flex items-center justify-center gap-2"
        >
          {submitting ? <Loader2 className="w-5 h-5 animate-spin" /> : <Plus className="w-5 h-5" />}
          {submitLabel}
        </button>
      </div>
    </div>
  );
}
