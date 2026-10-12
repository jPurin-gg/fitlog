"use client";

/* eslint-disable @typescript-eslint/no-explicit-any */
import React from "react";
import { AIToggle, useAISettings } from "@/components/AISettings";
import { motion } from "framer-motion";
import { Loader2, X, MessageSquare, BrainCircuit, ArrowRight, AlertCircle } from "lucide-react";
import { useBodyScrollLock } from "@/hooks/useBodyScrollLock";
import { apiErrorMessage, apiFetch } from "@/lib/api";

export function AlternativeCoachModal({ exerciseId, exerciseName, onClose, onReplace }: any) {
  const { enabled: aiEnabled, isEnabled } = useAISettings();
  const [loading, setLoading] = React.useState(false);
  const [reason, setReason] = React.useState("");
  const [customName, setCustomName] = React.useState("");
  const [result, setResult] = React.useState<any>(null);
  const [error, setError] = React.useState("");
  const [aiError, setAIError] = React.useState("");
  const [customLoading, setCustomLoading] = React.useState(false);
  const inFlightRef = React.useRef(false);
  const abortRef = React.useRef<AbortController | null>(null);
  const requestRef = React.useRef(0);

  React.useEffect(() => () => abortRef.current?.abort(), []);
  React.useEffect(() => {
    if (aiEnabled) return;
    abortRef.current?.abort();
    requestRef.current += 1;
    inFlightRef.current = false;
    setLoading(false);
    setResult(null);
    setAIError("");
  }, [aiEnabled]);

  useBodyScrollLock();

  const fetchAlternatives = async () => {
    if (!isEnabled() || inFlightRef.current) return;
    const controller = new AbortController();
    abortRef.current = controller;
    const requestID = ++requestRef.current;
    const current = () => isEnabled() && !controller.signal.aborted && requestRef.current === requestID;
    inFlightRef.current = true;
    setLoading(true);
    setAIError("");
    setResult(null);
    try {
      if (!exerciseId) {
        setAIError("種目IDがないため、AIに代替種目を相談できません。");
        return;
      }
      const data = await apiFetch<any>(`/api/exercises/${encodeURIComponent(exerciseId)}/alternatives`, {
        method: 'POST',
        signal: controller.signal,
        body: JSON.stringify({ reason })
      });
      if (current()) setResult(data);
    } catch (e) {
      if (current()) setAIError(apiErrorMessage(e, "AIに相談できませんでした。手動で変更できます。"));
    } finally {
      if (requestRef.current === requestID) {
        inFlightRef.current = false;
        setLoading(false);
      }
    }
  };

  const saveCustomExercise = async () => {
    const name = customName.trim();
    if (!name || customLoading) return;
    setCustomLoading(true);
    setError("");
    try {
      const created = await apiFetch<{ id: string; name: string }>("/api/exercises", {
        method: "POST",
        body: JSON.stringify({ name, category: "strength", equipment: "その他" }),
      });
      onReplace(created);
    } catch (e) {
      console.error(e);
      setError(apiErrorMessage(e, "種目の追加に失敗しました。"));
    } finally {
      setCustomLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[100] flex flex-col items-center justify-center p-4 overflow-hidden overscroll-none">
      <div className="absolute inset-0 bg-black/80 backdrop-blur-md" onClick={onClose} />
      
      <motion.div 
        role="dialog"
        aria-modal="true"
        aria-label="種目を変更"
        initial={{ opacity: 0, y: 20, scale: 0.95 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: 10, scale: 0.95 }}
        className="w-full max-w-lg bg-[#111] border border-white/10 rounded-[32px] p-6 pb-24 lg:p-10 lg:pb-10 relative z-10 shadow-2xl max-h-[85vh] overflow-y-auto overscroll-contain"
      >
        <button 
          onClick={onClose}
          className="absolute top-6 right-6 p-2 bg-white/5 rounded-full hover:bg-white/10 transition-colors"
        >
          <X className="w-5 h-5 text-white/50" />
        </button>

        <div className="flex items-center gap-3 mb-6">
          <div className="p-2 bg-primary/20 rounded-xl">
            <MessageSquare className="w-6 h-6 text-primary" />
          </div>
          <h2 className="text-xl font-bold tracking-tight">種目を変更</h2>
        </div>

        <div className="mb-4"><AIToggle /></div>
        {!aiEnabled && <p className="mb-4 text-sm text-white/50">AIはOFFです。種目は手動で変更できます。</p>}
        {error && <p role="alert" className="mb-4 text-sm text-red-300">{error}</p>}

        <div className="bg-white/5 rounded-2xl p-4 mb-6 border border-white/5 flex items-center justify-between">
          <div className="text-sm font-medium text-white/40">変更対象</div>
          <div className="text-lg font-black text-white">{exerciseName}</div>
        </div>

        {!aiEnabled || !result ? (
          <div className="space-y-6">
            <div className="space-y-3">
              <label className="text-[12px] font-black text-white/40 ml-1">マシンが空いていないですか？</label>
              <textarea 
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-sm min-h-[100px]"
                placeholder="例: マシンが埋まっているのでダンベルでやりたい、など（入力なしでも可）"
              />
            </div>
            
            <button 
              onClick={fetchAlternatives}
              disabled={loading || !aiEnabled}
              className="w-full py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-30 flex items-center justify-center gap-2"
            >
              {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <><BrainCircuit className="w-5 h-5" /> AIに代替種目を相談する</>}
            </button>

            {aiEnabled && aiError && (
              <div className="p-4 bg-red-500/10 border border-red-500/30 rounded-2xl flex items-start gap-3">
                <AlertCircle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-red-300 font-bold text-sm">エラーが発生しました</p>
                  <p className="text-red-200/70 text-xs mt-1 whitespace-pre-line">{aiError}</p>
                  <button
                    onClick={fetchAlternatives}
                    disabled={loading || !aiEnabled}
                    className="mt-3 text-xs text-red-300 hover:text-red-200 underline transition-colors disabled:opacity-50"
                  >
                    もう一度試す
                  </button>
                </div>
              </div>
            )}

            <div className="relative pt-6 pb-2">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-white/10"></div>
              </div>
              <div className="relative flex justify-center text-xs">
                <span className="bg-[#111] px-2 text-white/40 font-bold tracking-widest">手動で変更する</span>
              </div>
            </div>

            <div className="flex gap-2">
              <input 
                type="text" 
                value={customName}
                onChange={(e) => setCustomName(e.target.value)}
                className="flex-1 bg-black/40 border border-white/10 rounded-xl px-4 py-3 focus:outline-none focus:border-white/30 text-sm"
                placeholder="好きな種目名を入力"
              />
              <button 
                onClick={saveCustomExercise}
                disabled={!customName.trim() || customLoading}
                className="px-6 bg-white/10 text-white font-bold rounded-xl hover:bg-white/20 transition-colors disabled:opacity-30"
              >
                {customLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : "決定"}
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-6 animate-in fade-in zoom-in-95 duration-300">
            <div className="p-4 bg-primary/10 rounded-2xl border border-primary/20">
              <p className="text-sm font-medium text-white/90 leading-relaxed">{result.message}</p>
            </div>

            <div className="space-y-3">
              <h4 className="text-[10px] font-black text-white/40 tracking-widest pl-1">候補種目</h4>
              {result.alternatives.map((alt: any, idx: number) => (
                <button 
                  key={idx}
                  onClick={() => onReplace({ id: alt.id || `custom_${Date.now()}`, name: alt.name })}
                  className="w-full text-left p-4 bg-white/5 rounded-2xl border border-white/5 hover:border-primary/50 hover:bg-primary/5 transition-all group"
                >
                  <div className="flex justify-between items-center mb-1">
                    <span className="font-bold text-primary group-hover:text-primary transition-colors">{alt.name}</span>
                    <ArrowRight className="w-4 h-4 text-white/20 group-hover:text-primary transition-colors group-hover:translate-x-1" />
                  </div>
                  <p className="text-xs text-white/50">{alt.description}</p>
                </button>
              ))}
            </div>

            <div className="flex gap-2 mt-4">
              <input 
                type="text" 
                value={customName}
                onChange={(e) => setCustomName(e.target.value)}
                className="flex-1 bg-black/40 border border-white/10 rounded-xl px-4 py-3 focus:outline-none focus:border-white/30 text-sm"
                placeholder="リストにない種目を手動で入力"
              />
              <button 
                onClick={saveCustomExercise}
                disabled={!customName.trim() || customLoading}
                className="px-6 bg-white/10 text-white font-bold rounded-xl hover:bg-white/20 transition-colors disabled:opacity-30"
              >
                {customLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : "決定"}
              </button>
            </div>
          </div>
        )}
      </motion.div>
    </div>
  );
}
