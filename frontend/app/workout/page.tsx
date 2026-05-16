"use client";

import React from "react";
import { 
  Flame, 
  TrendingUp, 
  Sparkles,
  BrainCircuit,
  Loader2,
  Play,
  RefreshCw,
  Search
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import Link from "next/link";
import { AlternativeCoachModal } from "@/components/AlternativeCoachModal";
import { ExerciseSelectorModal } from "@/components/ExerciseSelectorModal";

export default function WorkoutPage() {
  const [loading, setLoading] = React.useState(false);
  const [recommendation, setRecommendation] = React.useState<any>(null);
  const [currentSet, setCurrentSet] = React.useState(1);
  const [formData, setFormData] = React.useState({ weight: '', reps: '', feeling: '' });
  const [workoutStarted, setWorkoutStarted] = React.useState(false);
  const [currentExercise, setCurrentExercise] = React.useState({
    id: "Barbell_Bench_Press_-_Medium_Grip", 
    name: "ベンチプレス"
  });
  const [showAltModal, setShowAltModal] = React.useState(false);
  const [showExerciseSelector, setShowExerciseSelector] = React.useState(false);
  const [targetSets, setTargetSets] = React.useState(3);
  const [editingTargetSets, setEditingTargetSets] = React.useState(false);
  const [tempTargetSets, setTempTargetSets] = React.useState(3);

  const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

  // 種目が変わったら目標セット数を取得
  React.useEffect(() => {
    const fetchTargetSets = async () => {
      try {
        const res = await fetch(`${apiUrl}/api/exercises/target_sets?user_id=1&exercise_id=${encodeURIComponent(currentExercise.id)}`);
        if (res.ok) {
          const data = await res.json();
          setTargetSets(data.target_sets ?? 3);
          setTempTargetSets(data.target_sets ?? 3);
        }
      } catch { /* デフォルト3セットのまま */ }
    };
    fetchTargetSets();
  }, [currentExercise.id]);

  const saveTargetSets = async (value: number) => {
    setTargetSets(value);
    setEditingTargetSets(false);
    try {
      await fetch(`${apiUrl}/api/exercises/target_sets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: 1, exercise_id: currentExercise.id, target_sets: value })
      });
    } catch { /* 無視 */ }
  };

  const startWorkout = () => {
    setWorkoutStarted(true);
  };

  const getRecommendation = async () => {
    if (!formData.weight || !formData.reps) return;
    setLoading(true);
    try {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
      const res = await fetch(`${apiUrl}/api/recommend`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          exercise_id: currentExercise.id, 
          user_id: 1, 
          set_order: currentSet,
          weight: parseFloat(formData.weight),
          reps: parseInt(formData.reps),
          feeling: formData.feeling
        })
      });
      const data = await res.json();
      setRecommendation(data);
    } catch (e) {
      console.error(e);
      // Fallback
      setRecommendation({
        next_action: "CONTINUE",
        recommendation: "素晴らしい！次のセットも維持しましょう。",
        target_weight: formData.weight,
        target_reps: formData.reps,
        reason: "安定した出力が維持されています。"
      });
    } finally {
      setLoading(false);
    }
  };

  const handleNextSet = () => {
    setCurrentSet(prev => prev + 1);
    setRecommendation(null);
    setFormData({ ...formData, feeling: '' }); // Reset only feeling
  };

  if (!workoutStarted) {
    return (
      <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 flex flex-col items-center justify-center relative overflow-hidden">
        <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
        <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

        <div className="z-10 text-center max-w-sm w-full mx-auto">
          <div className="w-24 h-24 bg-primary/20 rounded-full flex items-center justify-center mx-auto mb-8 relative">
            <div className="absolute inset-0 bg-primary/20 rounded-full animate-ping" />
            <Play className="w-10 h-10 text-primary ml-2" />
          </div>
          <h1 className="text-3xl font-bold mb-4">Ready to lift?</h1>
          <p className="text-white/50 mb-10">Start your AI-guided workout session and let the smart coach do the thinking.</p>
          <button 
            onClick={startWorkout}
            className="w-full py-5 bg-primary text-black font-black rounded-2xl shadow-[0_0_30px_rgba(255,170,0,0.5)] hover:scale-[1.02] active:scale-[0.98] transition-all duration-300 text-xl tracking-wider uppercase group relative overflow-hidden"
          >
            <div className="absolute inset-0 -translate-x-full group-hover:animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-transparent via-white/30 to-transparent skew-x-12" />
            <span className="relative z-10 flex items-center justify-center gap-2">
              Start Workout <Sparkles className="w-5 h-5" />
            </span>
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 relative selection:bg-primary/30">
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <AnimatePresence>
        {showAltModal && (
          <AlternativeCoachModal 
            exerciseId={currentExercise.id}
            exerciseName={currentExercise.name}
            onClose={() => setShowAltModal(false)}
            onReplace={(newEx: { id: string; name: string }) => {
              setCurrentExercise({ id: newEx.id, name: newEx.name });
              setCurrentSet(1);
              setRecommendation(null);
              setShowAltModal(false);
            }}
          />
        )}
      </AnimatePresence>

      {showExerciseSelector && (
        <ExerciseSelectorModal
          onClose={() => setShowExerciseSelector(false)}
          onSelect={(ex) => {
            setCurrentExercise(ex);
            setCurrentSet(1);
            setRecommendation(null);
            setShowExerciseSelector(false);
          }}
        />
      )}

      <main className="max-w-3xl mx-auto relative z-10">
        <header className="mb-8 flex justify-between items-center">
            <div>
              <h1 className="text-2xl font-bold tracking-tight">Active Workout</h1>
              <p className="text-primary font-medium text-sm flex items-center gap-2">
                 <BrainCircuit className="w-4 h-4" /> AI Coach Active
              </p>
            </div>
            <div className="text-right">
              <div className="px-4 py-1.5 glass rounded-full text-sm font-black text-white/50">
                00:00:00 {/* Ideally a running timer here! */}
              </div>
            </div>
        </header>

        <section className="glass rounded-[32px] p-6 lg:p-8 border-primary/20 bg-primary/5 overflow-hidden relative shadow-2xl shadow-primary/5">
          <div className="flex justify-between items-center mb-8 border-b border-white/5 pb-6">
            <div>
              <h2 className="text-2xl font-bold italic tracking-wider mb-2">{currentExercise.name}</h2>
              <div className="flex gap-2">
                <button 
                  onClick={() => setShowExerciseSelector(true)}
                  className="text-[10px] font-bold text-primary hover:text-primary/80 flex items-center gap-1 transition-colors bg-primary/10 hover:bg-primary/20 px-3 py-1.5 rounded-lg border border-primary/20"
                >
                  <Search className="w-3 h-3" /> 種目を検索して変更
                </button>
                <button 
                  onClick={() => setShowAltModal(true)}
                  className="text-[10px] font-bold text-white/50 hover:text-primary flex items-center gap-1 transition-colors bg-white/5 hover:bg-white/10 px-3 py-1.5 rounded-lg border border-white/10"
                >
                  <RefreshCw className="w-3 h-3" /> AIに代替種目を聞く
                </button>
              </div>
            </div>
          </div>
          {/* SET カウンター＆目標セット数 */}
          <div className="flex flex-col items-end gap-1">
            <div className="px-5 py-2 bg-primary/20 rounded-2xl text-sm font-black text-primary border border-primary/30">
              SET {currentSet} / {targetSets}
            </div>
            {/* 進捗バー */}
            <div className="flex gap-1">
              {Array.from({ length: targetSets }).map((_, i) => (
                <div
                  key={i}
                  className={`h-1.5 w-5 rounded-full transition-colors ${
                    i < currentSet ? 'bg-primary' : 'bg-white/10'
                  }`}
                />
              ))}
            </div>
            {/* 目標セット数の編集 */}
            {editingTargetSets ? (
              <div className="flex items-center gap-1 mt-1">
                <button onClick={() => saveTargetSets(Math.max(1, tempTargetSets - 1))} className="w-6 h-6 bg-white/10 rounded-lg text-white font-bold hover:bg-white/20 transition-colors flex items-center justify-center text-xs">−</button>
                <span className="text-xs text-white/70 w-10 text-center font-bold">{tempTargetSets} セット</span>
                <button onClick={() => saveTargetSets(Math.min(20, tempTargetSets + 1))} className="w-6 h-6 bg-white/10 rounded-lg text-white font-bold hover:bg-white/20 transition-colors flex items-center justify-center text-xs">＋</button>
                <button onClick={() => setEditingTargetSets(false)} className="text-[10px] text-white/40 ml-1 hover:text-white/60">✕</button>
              </div>
            ) : (
              <button
                onClick={() => { setTempTargetSets(targetSets); setEditingTargetSets(true); }}
                className="text-[10px] text-white/30 hover:text-white/60 transition-colors mt-0.5"
              >
                目標セット数を変更
              </button>
            )}
          </div>

          <div className="grid grid-cols-1 gap-8">
            {/* Input Form */}
            <div className="space-y-6">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <label className="text-[10px] uppercase font-black text-white/40 ml-1">Weight (kg)</label>
                  <input 
                    type="number" 
                    value={formData.weight}
                    onChange={(e) => setFormData({...formData, weight: e.target.value})}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-2xl font-bold"
                    placeholder="80"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-[10px] uppercase font-black text-white/40 ml-1">Reps</label>
                  <input 
                    type="number" 
                    value={formData.reps}
                    onChange={(e) => setFormData({...formData, reps: e.target.value})}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-secondary/50 text-2xl font-bold"
                    placeholder="10"
                  />
                </div>
              </div>
              <div className="space-y-2">
                <label className="text-[10px] uppercase font-black text-white/40 ml-1">Set Feeling / Impression</label>
                <textarea 
                  value={formData.feeling}
                  onChange={(e) => setFormData({...formData, feeling: e.target.value})}
                  className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-accent/50 text-sm min-h-[100px]"
                  placeholder="例: きつかった、まだ余裕がある、肩に違和感..."
                />
              </div>

              {!recommendation && (
                <button 
                  onClick={getRecommendation}
                  disabled={loading || !formData.weight || !formData.reps}
                  className="w-full py-5 mt-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-30 disabled:pointer-events-none uppercase tracking-widest flex items-center justify-center gap-2 shadow-[0_0_20px_rgba(255,170,0,0.3)]"
                >
                  {loading ? <Loader2 className="w-6 h-6 animate-spin" /> : "Set Completed - Analyze"}
                </button>
              )}
            </div>

            {/* AI Result Area - Only shows when recommendation exists */}
            {recommendation && (
              <div className="animate-in slide-in-from-bottom-8 fade-in duration-500 pt-6 border-t border-white/5">
                <div className="space-y-6">
                  <div className={`p-6 rounded-[24px] border-2 flex items-center justify-between ${
                    recommendation.next_action === 'STOP' 
                    ? 'bg-accent/10 border-accent/30 text-accent' 
                    : 'bg-primary/10 border-primary/30 text-primary'
                  }`}>
                    <div className="flex items-center gap-3">
                      {recommendation.next_action === 'STOP' ? <Flame className="w-8 h-8" /> : <TrendingUp className="w-8 h-8" />}
                      <span className="text-3xl font-black italic tracking-tighter uppercase">
                        {recommendation.next_action === 'STOP' ? 'STOP NOW' : 'NEXT SET: CONTINUE'}
                      </span>
                    </div>
                  </div>

                  <div className="glass bg-black/50 p-6 rounded-[24px] border-white/5 space-y-4">
                    <div className="flex items-center gap-2 text-primary font-bold text-sm">
                      <Sparkles className="w-4 h-4" /> AI COACH SAYS
                    </div>
                    <p className="text-white/90 leading-relaxed font-medium text-lg">
                      {recommendation.recommendation}
                    </p>
                    {recommendation.next_action !== 'STOP' && (
                      <div className="flex gap-4 pt-4">
                        <div className="flex-1 bg-white/5 p-4 rounded-xl border border-white/5 text-center">
                          <span className="text-[10px] uppercase font-black text-white/40 block mb-1">Target Kg</span>
                          <span className="text-2xl font-bold">{recommendation.target_weight}</span>
                        </div>
                        <div className="flex-1 bg-white/5 p-4 rounded-xl border border-white/5 text-center">
                          <span className="text-[10px] uppercase font-black text-white/40 block mb-1">Target Reps</span>
                          <span className="text-2xl font-bold">{recommendation.target_reps}</span>
                        </div>
                      </div>
                    )}
                    <div className="pt-4 border-t border-white/5 flex justify-between items-end">
                      <div>
                        <p className="text-[10px] text-white/30 uppercase font-black mb-1">Coach's Rationale</p>
                        <p className="text-xs text-white/50 italic">{recommendation.reason}</p>
                      </div>
                      <div className="text-right">
                        <p className="text-[10px] text-white/30 uppercase font-black mb-1">Personal Best</p>
                        <p className="text-sm font-bold text-white/70">{recommendation.max_weight}<span className="text-[10px] ml-0.5">kg</span></p>
                      </div>
                    </div>
                  </div>

                  <div className="pt-2">
                    {recommendation.next_action !== 'STOP' ? (
                      <button 
                        onClick={handleNextSet}
                        className="w-full py-4 bg-white/10 hover:bg-white/20 text-white font-bold rounded-2xl transition-all"
                      >
                        Start Next Set
                      </button>
                    ) : (
                      <button 
                        onClick={() => {setWorkoutStarted(false); setRecommendation(null); setCurrentSet(1); setFormData({weight:'', reps:'', feeling:''})}}
                        className="w-full py-4 bg-accent/20 hover:bg-accent/30 text-accent font-bold rounded-2xl transition-all"
                      >
                        Finish Exercise & Workout
                      </button>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}
