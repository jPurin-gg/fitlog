"use client";

/* eslint-disable @typescript-eslint/no-explicit-any */
import React from "react";
import { motion } from "framer-motion";
import { Loader2, X, MessageSquare, BrainCircuit, ArrowRight, AlertCircle } from "lucide-react";

export function AlternativeCoachModal({ exerciseId, exerciseName, onClose, onReplace }: any) {
  const [loading, setLoading] = React.useState(false);
  const [reason, setReason] = React.useState("");
  const [customName, setCustomName] = React.useState("");
  const [result, setResult] = React.useState<any>(null);
  const [error, setError] = React.useState("");
  const inFlightRef = React.useRef(false);

  React.useEffect(() => {
    const previousOverflow = document.body.style.overflow;
    const previousTouchAction = document.body.style.touchAction;
    document.body.style.overflow = "hidden";
    document.body.style.touchAction = "none";
    return () => {
      document.body.style.overflow = previousOverflow;
      document.body.style.touchAction = previousTouchAction;
    };
  }, []);

  const fetchAlternatives = async () => {
    if (inFlightRef.current) return;
    inFlightRef.current = true;
    setLoading(true);
    setError("");
    setResult(null);
    try {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || '';
      const res = await fetch(`${apiUrl}/api/alternative`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ exercise_id: exerciseId, exercise: exerciseName, reason: reason })
      });
      if (!res.ok) {
        setError(await res.text() || "AIによる代替種目の提案に失敗しました。");
        return;
      }
      const data = await res.json();
      setResult(data);
    } catch (e) {
      console.error(e);
      setError("ネットワークエラーが発生しました。バックエンドに接続できません。");
    } finally {
      inFlightRef.current = false;
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[100] flex flex-col items-center justify-center p-4 overflow-hidden overscroll-none">
      <div className="absolute inset-0 bg-black/80 backdrop-blur-md" onClick={onClose} />
      
      <motion.div 
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

        <div className="bg-white/5 rounded-2xl p-4 mb-6 border border-white/5 flex items-center justify-between">
          <div className="text-sm font-medium text-white/40">変更対象</div>
          <div className="text-lg font-black text-white">{exerciseName}</div>
        </div>

        {!result ? (
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
              disabled={loading}
              className="w-full py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-30 flex items-center justify-center gap-2"
            >
              {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <><BrainCircuit className="w-5 h-5" /> AIに代替種目を相談する</>}
            </button>

            {error && (
              <div className="p-4 bg-red-500/10 border border-red-500/30 rounded-2xl flex items-start gap-3">
                <AlertCircle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-red-300 font-bold text-sm">エラーが発生しました</p>
                  <p className="text-red-200/70 text-xs mt-1 whitespace-pre-line">{error}</p>
                  <button
                    onClick={fetchAlternatives}
                    disabled={loading}
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
                onClick={() => customName && onReplace(customName)}
                disabled={!customName}
                className="px-6 bg-white/10 text-white font-bold rounded-xl hover:bg-white/20 transition-colors disabled:opacity-30"
              >
                決定
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
                onClick={() => customName && onReplace({ id: `custom_${Date.now()}`, name: customName })}
                disabled={!customName}
                className="px-6 bg-white/10 text-white font-bold rounded-xl hover:bg-white/20 transition-colors disabled:opacity-30"
              >
                決定
              </button>
            </div>
          </div>
        )}
      </motion.div>
    </div>
  );
}
