"use client";

import React from "react";
import { Loader2, Pencil, Plus, Scale, Trash2, X } from "lucide-react";
import { apiErrorMessage, apiFetch } from "@/lib/api";

export interface BodyMeasurement {
  id: number;
  measured_on: string;
  weight_kg: number;
  body_fat_percent?: number | null;
  resting_heart_rate?: number | null;
}

function todayString() {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}

export default function BodyMeasurements({ onChanged }: { onChanged?: () => void }) {
  const [measurements, setMeasurements] = React.useState<BodyMeasurement[] | null>(null);
  const [editingId, setEditingId] = React.useState<number | null>(null);
  const [showForm, setShowForm] = React.useState(false);
  const [measuredOn, setMeasuredOn] = React.useState(todayString());
  const [weight, setWeight] = React.useState("");
  const [bodyFat, setBodyFat] = React.useState("");
  const [heartRate, setHeartRate] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");

  const load = React.useCallback(async (signal?: AbortSignal) => {
    try {
      const data = await apiFetch<BodyMeasurement[]>("/api/body-measurements", { signal });
      setMeasurements(Array.isArray(data) ? data : []);
    } catch (err) {
      if ((err as Error)?.name === "AbortError") return;
      console.error(err);
      setMeasurements([]);
    }
  }, []);

  React.useEffect(() => {
    const controller = new AbortController();
    load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const resetForm = () => {
    setEditingId(null);
    setShowForm(false);
    setMeasuredOn(todayString());
    setWeight("");
    setBodyFat("");
    setHeartRate("");
    setError("");
  };

  const startEdit = (measurement: BodyMeasurement) => {
    setEditingId(measurement.id);
    setShowForm(true);
    setMeasuredOn(measurement.measured_on);
    setWeight(String(measurement.weight_kg));
    setBodyFat(measurement.body_fat_percent != null ? String(measurement.body_fat_percent) : "");
    setHeartRate(measurement.resting_heart_rate != null ? String(measurement.resting_heart_rate) : "");
    setError("");
  };

  const save = async () => {
    setSaving(true);
    setError("");
    const payload: Record<string, unknown> = {
      measured_on: measuredOn,
      weight_kg: Number(weight),
    };
    if (bodyFat !== "") payload.body_fat_percent = Number(bodyFat);
    if (heartRate !== "") payload.resting_heart_rate = Number(heartRate);
    try {
      if (editingId != null) {
        await apiFetch(`/api/body-measurements/${editingId}`, { method: "PUT", body: JSON.stringify(payload) });
      } else {
        await apiFetch("/api/body-measurements", { method: "POST", body: JSON.stringify(payload) });
      }
      resetForm();
      await load();
      onChanged?.();
    } catch (err) {
      console.error(err);
      setError(apiErrorMessage(err, "身体データの保存に失敗しました。"));
    } finally {
      setSaving(false);
    }
  };

  const remove = async (measurement: BodyMeasurement) => {
    if (!window.confirm(`${measurement.measured_on} の記録を削除しますか？`)) return;
    try {
      await apiFetch(`/api/body-measurements/${measurement.id}`, { method: "DELETE" });
      if (editingId === measurement.id) resetForm();
      await load();
      onChanged?.();
    } catch (err) {
      console.error(err);
      setError(apiErrorMessage(err, "身体データの削除に失敗しました。"));
    }
  };

  const inputClass = "w-full bg-black/40 border border-white/10 rounded-xl px-3 py-2.5 text-sm text-white focus:outline-none focus:border-primary/60";

  return (
    <section className="glass rounded-[32px] p-6 md:p-10 border border-white/5">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-bold flex items-center gap-2">
          <Scale className="w-5 h-5 text-secondary" /> 身体データ
        </h2>
        {!showForm && (
          <button
            onClick={() => { resetForm(); setShowForm(true); }}
            className="flex items-center gap-1.5 text-sm font-bold text-primary bg-primary/10 px-4 py-2 rounded-xl border border-primary/20 hover:bg-primary/20 transition-colors"
          >
            <Plus className="w-4 h-4" /> 記録する
          </button>
        )}
      </div>

      {error && <p className="text-sm text-red-400 mb-4">{error}</p>}

      {showForm && (
        <div className="bg-white/5 rounded-2xl border border-white/10 p-5 mb-6">
          <div className="flex justify-between items-center mb-4">
            <h3 className="font-bold">{editingId != null ? "記録を編集" : "新しい記録"}</h3>
            <button onClick={resetForm} className="text-white/40 hover:text-white" aria-label="閉じる">
              <X className="w-4 h-4" />
            </button>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            <label className="block">
              <span className="text-[10px] font-bold text-white/40 block mb-1">日付</span>
              <input type="date" value={measuredOn} onChange={e => setMeasuredOn(e.target.value)} className={inputClass} />
            </label>
            <label className="block">
              <span className="text-[10px] font-bold text-white/40 block mb-1">体重 (kg)</span>
              <input type="number" step="0.1" min="0" value={weight} onChange={e => setWeight(e.target.value)} placeholder="68.5" className={inputClass} />
            </label>
            <label className="block">
              <span className="text-[10px] font-bold text-white/40 block mb-1">体脂肪率 (%) 任意</span>
              <input type="number" step="0.1" min="0" max="100" value={bodyFat} onChange={e => setBodyFat(e.target.value)} placeholder="15.0" className={inputClass} />
            </label>
            <label className="block">
              <span className="text-[10px] font-bold text-white/40 block mb-1">安静時心拍数 (bpm) 任意</span>
              <input type="number" step="1" min="0" value={heartRate} onChange={e => setHeartRate(e.target.value)} placeholder="72" className={inputClass} />
            </label>
          </div>
          <button
            onClick={save}
            disabled={saving || weight === "" || measuredOn === ""}
            className="px-6 py-2.5 bg-primary text-black font-bold rounded-xl disabled:opacity-40 flex items-center gap-2"
          >
            {saving && <Loader2 className="w-4 h-4 animate-spin" />} 保存する
          </button>
        </div>
      )}

      {measurements == null ? (
        <div className="text-center py-6 text-white/30 text-sm flex items-center justify-center gap-2">
          <Loader2 className="w-4 h-4 animate-spin" /> 記録を読み込み中...
        </div>
      ) : measurements.length === 0 ? (
        <div className="text-center py-8 text-white/40 text-sm bg-white/5 rounded-2xl border border-white/5">
          まだ身体データの記録はありません。「記録する」から体重などを追加できます。
        </div>
      ) : (
        <ul className="divide-y divide-white/5">
          {measurements.map(measurement => (
            <li key={measurement.id} className="flex items-center gap-3 py-3">
              <span className="text-sm font-bold w-24 flex-shrink-0">{measurement.measured_on}</span>
              <span className="text-sm">{measurement.weight_kg} kg</span>
              {measurement.body_fat_percent != null && (
                <span className="text-xs text-white/40">体脂肪率 {measurement.body_fat_percent}%</span>
              )}
              {measurement.resting_heart_rate != null && (
                <span className="text-xs text-white/40">心拍数 {measurement.resting_heart_rate} bpm</span>
              )}
              <span className="ml-auto flex items-center gap-1">
                <button onClick={() => startEdit(measurement)} className="p-2 text-white/40 hover:text-primary" aria-label="編集">
                  <Pencil className="w-4 h-4" />
                </button>
                <button onClick={() => remove(measurement)} className="p-2 text-white/40 hover:text-red-400" aria-label="削除">
                  <Trash2 className="w-4 h-4" />
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
