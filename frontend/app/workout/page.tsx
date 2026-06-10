"use client";

/* eslint-disable @typescript-eslint/no-explicit-any */
import React from "react";
import { 
  Flame, 
  TrendingUp, 
  Sparkles,
  BrainCircuit,
  Loader2,
  Play,
  RefreshCw,
  Search,
  CheckCircle2,
  ListChecks,
  Trophy,
  RotateCcw,
  Home
} from "lucide-react";
import { AnimatePresence } from "framer-motion";
import Link from "next/link";
import { AlternativeCoachModal } from "@/components/AlternativeCoachModal";
import { ExerciseSelectorModal } from "@/components/ExerciseSelectorModal";
import { WorkoutSummaryView, type WorkoutSummary } from "@/components/WorkoutSummaryView";
import { useAuth } from "@/components/AuthGate";

interface WorkoutPlanExercise {
  exercise_id: string;
  name: string;
  planned_sets: number;
  target_weight: number;
  target_reps: number;
  last_max_weight?: number;
}

interface WorkoutPlanSession {
  id: number;
  workout_id: number;
  user_id: number;
  plan_date: string;
  status: string;
  plan: {
    workout_title: string;
    target: string;
    estimated_duration_min: number;
    coach_note: string;
    exercises: WorkoutPlanExercise[];
  };
}

function displayPlanText(text?: string) {
  if (!text) return "";
  return text
    .replace("PPL法 (Push/Pull/Legs)", "PPL法（押す・引く・脚）")
    .replace("Full Body", "全身")
    .replace("Push (胸・肩・三頭)", "押す日（胸・肩・三頭）")
    .replace("Pull (背中・二頭)", "引く日（背中・二頭）")
    .replace("Legs (脚・腹)", "脚の日（脚・腹）")
    .replace("全身 A", "全身その1")
    .replace("全身 B", "全身その2");
}

export default function WorkoutPage() {
  const { user } = useAuth();
  const [loading, setLoading] = React.useState(false);
  const [finishing, setFinishing] = React.useState(false);
  const [recommendation, setRecommendation] = React.useState<any>(null);
  const [aiError, setAiError] = React.useState<string | null>(null);
  const [currentSet, setCurrentSet] = React.useState(1);
  const [formData, setFormData] = React.useState({ weight: '', reps: '', feeling: '' });
  const [workoutStarted, setWorkoutStarted] = React.useState(false);
  const [currentExercise, setCurrentExercise] = React.useState({
    id: "Barbell_Bench_Press_-_Medium_Grip", 
    name: "ベンチプレス"
  });
  const [workoutPlan, setWorkoutPlan] = React.useState<WorkoutPlanSession | null>(null);
  const [currentExerciseIndex, setCurrentExerciseIndex] = React.useState(0);
  const [showAltModal, setShowAltModal] = React.useState(false);
  const [showExerciseSelector, setShowExerciseSelector] = React.useState(false);
  const [targetSets, setTargetSets] = React.useState(3);
  const [editingTargetSets, setEditingTargetSets] = React.useState(false);
  const [tempTargetSets, setTempTargetSets] = React.useState(3);
  const [finishedSummary, setFinishedSummary] = React.useState<WorkoutSummary | null>(null);
  const startInFlightRef = React.useRef(false);
  const recommendationInFlightRef = React.useRef(false);
  const finishInFlightRef = React.useRef(false);

  const apiUrl = process.env.NEXT_PUBLIC_API_URL || '';

  // 種目が変わったら目標セット数を取得
  React.useEffect(() => {
    const fetchTargetSets = async () => {
      if (workoutPlan) return;
      try {
        const res = await fetch(`${apiUrl}/api/exercises/target_sets?user_id=${user.id}&exercise_id=${encodeURIComponent(currentExercise.id)}`);
        if (res.ok) {
          const data = await res.json();
          setTargetSets(data.target_sets ?? 3);
          setTempTargetSets(data.target_sets ?? 3);
        }
      } catch { /* デフォルト3セットのまま */ }
    };
    fetchTargetSets();
  }, [apiUrl, currentExercise.id, user.id, workoutPlan]);

  const saveTargetSets = async (value: number) => {
    setTargetSets(value);
    setEditingTargetSets(false);
    try {
      await fetch(`${apiUrl}/api/exercises/target_sets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: user.id, exercise_id: currentExercise.id, target_sets: value })
      });
    } catch { /* 無視 */ }
  };

  const activatePlanExercise = (plan: WorkoutPlanSession, index: number) => {
    const ex = plan.plan.exercises[index];
    if (!ex) return;
    setCurrentExerciseIndex(index);
    setCurrentExercise({ id: ex.exercise_id, name: ex.name });
    setTargetSets(ex.planned_sets || 3);
    setTempTargetSets(ex.planned_sets || 3);
    setCurrentSet(1);
    setRecommendation(null);
    setAiError(null);
    setFormData({
      weight: ex.target_weight ? String(ex.target_weight) : '',
      reps: ex.target_reps ? String(ex.target_reps) : '',
      feeling: ''
    });
  };

  const startWorkout = async () => {
    if (startInFlightRef.current) return;
    startInFlightRef.current = true;
    setLoading(true);
    setAiError(null);
    setFinishedSummary(null);
    try {
      const res = await fetch(`${apiUrl}/api/workout-plan/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: user.id })
      });
      if (!res.ok) {
        setAiError(await res.text() || '今日の計画作成に失敗しました。');
        return;
      }
      const data = await res.json();
      setWorkoutPlan(data);
      setWorkoutStarted(true);
      activatePlanExercise(data, 0);
    } catch (e) {
      console.error(e);
      setAiError('バックエンドに接続できません。');
    } finally {
      startInFlightRef.current = false;
      setLoading(false);
    }
  };

  const getRecommendation = async () => {
    if (!formData.weight || !formData.reps) return;
    if (recommendationInFlightRef.current) return;
    recommendationInFlightRef.current = true;
    setLoading(true);
    setAiError(null);
    try {
      const res = await fetch(`${apiUrl}/api/recommend`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          exercise_id: currentExercise.id, 
          workout_id: workoutPlan?.workout_id || 0,
          user_id: user.id,
          set_order: currentSet,
          weight: parseFloat(formData.weight),
          reps: parseInt(formData.reps),
          feeling: formData.feeling
        })
      });
      if (!res.ok) {
        const msg = await res.text();
        setAiError(msg || 'AIの呼び出しに失敗しました。');
        return;
      }
      const data = await res.json();
      setRecommendation(data);
    } catch (e) {
      console.error(e);
      setAiError('ネットワークエラーが発生しました。バックエンドに接続できません。');
    } finally {
      recommendationInFlightRef.current = false;
      setLoading(false);
    }
  };

  const handleNextSet = () => {
    setCurrentSet(prev => prev + 1);
    if (recommendation?.target_weight) {
      setFormData({
        weight: String(recommendation.target_weight),
        reps: recommendation.target_reps ? String(recommendation.target_reps) : formData.reps,
        feeling: ''
      });
    } else {
      setFormData({ ...formData, feeling: '' });
    }
    setRecommendation(null);
    setAiError(null);
  };

  const handleNextExercise = () => {
    if (!workoutPlan) return;
    const nextIndex = currentExerciseIndex + 1;
    if (nextIndex >= workoutPlan.plan.exercises.length) {
      finishWorkout();
      return;
    }
    activatePlanExercise(workoutPlan, nextIndex);
  };

  const finishWorkout = async () => {
    if (finishInFlightRef.current) return;
    finishInFlightRef.current = true;
    setFinishing(true);
    setAiError(null);
    try {
      const res = await fetch(`${apiUrl}/api/workouts/finish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: user.id, workout_id: workoutPlan?.workout_id || 0 })
      });
      if (!res.ok) {
        setAiError(await res.text() || 'ワークアウト終了に失敗しました。');
        return;
      }
      const data = await res.json();
      setFinishedSummary(data.summary || null);
      setWorkoutStarted(false);
      setWorkoutPlan(null);
      setRecommendation(null);
      setCurrentSet(1);
      setCurrentExerciseIndex(0);
      setFormData({ weight: '', reps: '', feeling: '' });
    } catch (e) {
      console.error(e);
      setAiError('バックエンドに接続できません。');
    } finally {
      finishInFlightRef.current = false;
      setFinishing(false);
    }
  };

  if (finishedSummary && !workoutStarted) {
    return (
      <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 relative overflow-hidden">
        <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
        <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

        <main className="max-w-3xl mx-auto relative z-10">
          <div className="mb-8">
            <div className="w-16 h-16 bg-primary/20 rounded-2xl flex items-center justify-center mb-5">
              <Trophy className="w-8 h-8 text-primary" />
            </div>
            <h1 className="text-3xl font-bold tracking-tight mb-2">今日のまとめ</h1>
            <p className="text-white/50">おつかれさまでした。今日の積み上げを記録しました。</p>
          </div>

          <WorkoutSummaryView
            summary={finishedSummary}
            emptyText="セット記録はまだありません。次回は1セット目から記録していきましょう。"
            emptyStateClassName="text-sm text-white/45"
            exerciseSectionClassName="mb-6"
          />

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <button
              onClick={() => setFinishedSummary(null)}
              className="py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center justify-center gap-2"
            >
              <RotateCcw className="w-5 h-5" /> 新しく始める
            </button>
            <Link
              href="/"
              className="py-4 bg-white/10 text-white font-bold rounded-2xl hover:bg-white/15 transition-all flex items-center justify-center gap-2"
            >
              <Home className="w-5 h-5" /> ホームへ
            </Link>
          </div>
        </main>
      </div>
    );
  }

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
          <h1 className="text-3xl font-bold mb-4">今日の筋トレを始めますか？</h1>
          <p className="text-white/50 mb-10">AIが今日のメニューを組み立て、セットごとに調整します。</p>
          <button 
            onClick={startWorkout}
            disabled={loading}
            className="w-full py-5 bg-primary text-black font-black rounded-2xl shadow-[0_0_30px_rgba(255,170,0,0.5)] hover:scale-[1.02] active:scale-[0.98] transition-all duration-300 text-xl tracking-wider group relative overflow-hidden disabled:opacity-50 disabled:pointer-events-none"
          >
            <div className="absolute inset-0 -translate-x-full group-hover:animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-transparent via-white/30 to-transparent skew-x-12" />
            <span className="relative z-10 flex items-center justify-center gap-2">
              {loading ? <Loader2 className="w-6 h-6 animate-spin" /> : <>ワークアウトを開始 <Sparkles className="w-5 h-5" /></>}
            </span>
          </button>
          {aiError && (
            <div className="mt-5 p-4 bg-red-500/10 border border-red-500/30 rounded-2xl text-left">
              <p className="text-red-300 font-bold text-sm">エラーが発生しました</p>
              <p className="text-red-200/70 text-xs mt-1 whitespace-pre-line">{aiError}</p>
              <button
                onClick={startWorkout}
                disabled={loading}
                className="mt-3 text-xs text-red-300 hover:text-red-200 underline transition-colors disabled:opacity-50 flex items-center gap-1"
              >
                <RefreshCw className="w-3 h-3" /> もう一度試す
              </button>
            </div>
          )}
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
            onReplace={(newExercise: string | { id?: string; name?: string }) => {
              const nextExercise = typeof newExercise === "string"
                ? { id: `custom_${Date.now()}`, name: newExercise }
                : { id: newExercise.id || `custom_${Date.now()}`, name: newExercise.name || currentExercise.name };
              setCurrentExercise({ id: nextExercise.id, name: nextExercise.name });
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
        <header className="mb-8 flex flex-col gap-4 sm:flex-row sm:justify-between sm:items-center">
            <div>
              <h1 className="text-2xl font-bold tracking-tight">ワークアウト中</h1>
              <p className="text-primary font-medium text-sm flex items-center gap-2">
                 <BrainCircuit className="w-4 h-4" /> AIコーチ稼働中
              </p>
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={finishWorkout}
                disabled={finishing}
                className="px-4 py-2 bg-white/5 hover:bg-accent/20 border border-white/10 hover:border-accent/30 rounded-xl text-xs font-bold text-white/60 hover:text-accent transition-colors disabled:opacity-50"
              >
                {finishing ? "終了中..." : "終了"}
              </button>
            </div>
        </header>

        <section className="glass rounded-[32px] p-6 lg:p-8 border-primary/20 bg-primary/5 overflow-hidden relative shadow-2xl shadow-primary/5">
          <div className="flex justify-between items-center mb-8 border-b border-white/5 pb-6">
            <div>
              <h2 className="text-2xl font-bold tracking-wider mb-2">{currentExercise.name}</h2>
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
              セット {currentSet} / {targetSets}
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

          {workoutPlan && (
            <div className="mb-8 bg-black/30 rounded-2xl border border-white/5 p-4">
              <div className="flex items-start justify-between gap-4 mb-4">
                <div>
                  <p className="text-[10px] tracking-widest text-primary font-black mb-1">今日の計画</p>
                  <h3 className="text-lg font-bold">{displayPlanText(workoutPlan.plan.workout_title)}</h3>
                  <p className="text-xs text-white/45 mt-1">{workoutPlan.plan.coach_note}</p>
                </div>
                <div className="text-right text-xs text-white/50 font-bold">
                  約 {workoutPlan.plan.estimated_duration_min} 分
                </div>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {workoutPlan.plan.exercises.map((ex, idx) => (
                  <button
                    key={`${ex.exercise_id}-${idx}`}
                    onClick={() => activatePlanExercise(workoutPlan, idx)}
                    className={`text-left p-3 rounded-xl border transition-all ${
                      idx === currentExerciseIndex
                        ? 'bg-primary/15 border-primary/40 text-white'
                        : 'bg-white/5 border-white/5 text-white/55 hover:bg-white/10 hover:text-white'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      {idx < currentExerciseIndex ? <CheckCircle2 className="w-4 h-4 text-green-400" /> : <ListChecks className="w-4 h-4 text-primary" />}
                      <span className="text-sm font-bold truncate">{ex.name}</span>
                    </div>
                    <div className="mt-1 text-[10px] text-white/40">
                      {ex.planned_sets}セット / {ex.target_weight}kg × {ex.target_reps}回
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="grid grid-cols-1 gap-8">
            {/* Input Form */}
            <div className="space-y-6">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <label className="text-[10px] font-black text-white/40 ml-1">重量（kg）</label>
                  <input 
                    type="number" 
                    value={formData.weight}
                    onChange={(e) => setFormData({...formData, weight: e.target.value})}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-2xl font-bold"
                    placeholder="80"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-[10px] font-black text-white/40 ml-1">回数</label>
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
                <label className="text-[10px] font-black text-white/40 ml-1">セット後の感想</label>
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
                  className="w-full py-5 mt-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-30 disabled:pointer-events-none tracking-widest flex items-center justify-center gap-2 shadow-[0_0_20px_rgba(255,170,0,0.3)]"
                >
                  {loading ? <Loader2 className="w-6 h-6 animate-spin" /> : "セット完了・AIに相談"}
                </button>
              )}

              {/* AI エラー表示 */}
              {aiError && !recommendation && (
                <div className="mt-4 p-4 bg-red-500/10 border border-red-500/30 rounded-2xl flex items-start gap-3">
                  <Flame className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-red-400 font-bold text-sm">AIの呼び出しに失敗しました</p>
                    <p className="text-red-300/70 text-xs mt-1">{aiError}</p>
                    <button
                      onClick={getRecommendation}
                      disabled={loading}
                      className="mt-3 text-xs text-red-400 hover:text-red-300 underline transition-colors"
                    >
                      もう一度試す
                    </button>
                  </div>
                </div>
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
                      <span className="text-3xl font-black tracking-tighter">
                        {recommendation.next_action === 'STOP' ? 'ここで中止推奨' : '次のセットへ'}
                      </span>
                    </div>
                  </div>

                  <div className="glass bg-black/50 p-6 rounded-[24px] border-white/5 space-y-4">
                    <div className="flex items-center gap-2 text-primary font-bold text-sm">
                      <Sparkles className="w-4 h-4" /> AIコーチの提案
                    </div>
                    <p className="text-white/90 leading-relaxed font-medium text-lg">
                      {recommendation.recommendation}
                    </p>
                    {recommendation.next_action !== 'STOP' && (
                      <div className="flex gap-4 pt-4">
                        <div className="flex-1 bg-white/5 p-4 rounded-xl border border-white/5 text-center">
                          <span className="text-[10px] font-black text-white/40 block mb-1">目標重量</span>
                          <span className="text-2xl font-bold">{recommendation.target_weight}</span>
                        </div>
                        <div className="flex-1 bg-white/5 p-4 rounded-xl border border-white/5 text-center">
                          <span className="text-[10px] font-black text-white/40 block mb-1">目標回数</span>
                          <span className="text-2xl font-bold">{recommendation.target_reps}</span>
                        </div>
                      </div>
                    )}
                    <div className="pt-4 border-t border-white/5 flex justify-between items-end">
                      <div>
                        <p className="text-[10px] text-white/30 font-black mb-1">提案理由</p>
                        <p className="text-xs text-white/50">{recommendation.reason}</p>
                      </div>
                      <div className="text-right">
                        <p className="text-[10px] text-white/30 font-black mb-1">過去最高</p>
                        <p className="text-sm font-bold text-white/70">{recommendation.max_weight}<span className="text-[10px] ml-0.5">kg</span></p>
                      </div>
                    </div>
                    {recommendation.record_template && (
                      <div className="pt-4 border-t border-white/5">
                        <p className="text-[10px] text-white/30 font-black mb-2">記録テンプレート</p>
                        <pre className="whitespace-pre-wrap rounded-xl border border-white/10 bg-black/50 p-3 text-xs leading-relaxed text-white/70">
                          {recommendation.record_template}
                        </pre>
                      </div>
                    )}
                  </div>

                  <div className="pt-2">
                    {recommendation.next_action !== 'STOP' ? (
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                        <button 
                          onClick={handleNextSet}
                          className="w-full py-4 bg-white/10 hover:bg-white/20 text-white font-bold rounded-2xl transition-all"
                        >
                          次のセットへ
                        </button>
                        <button 
                          onClick={handleNextExercise}
                          className="w-full py-4 bg-primary/20 hover:bg-primary/30 text-primary font-bold rounded-2xl transition-all"
                        >
                          {currentExerciseIndex + 1 >= (workoutPlan?.plan.exercises.length || 1) ? 'ワークアウト終了' : '次の種目へ'}
                        </button>
                      </div>
                    ) : (
                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                        <button 
                          onClick={handleNextSet}
                          className="w-full py-4 bg-white/10 hover:bg-white/20 text-white font-bold rounded-2xl transition-all"
                        >
                          もう1セットだけ続ける
                        </button>
                        <button 
                          onClick={handleNextExercise}
                          className="w-full py-4 bg-primary/20 hover:bg-primary/30 text-primary font-bold rounded-2xl transition-all"
                        >
                          次の種目へ
                        </button>
                        <button 
                          onClick={finishWorkout}
                          disabled={finishing}
                          className="w-full py-4 bg-accent/20 hover:bg-accent/30 text-accent font-bold rounded-2xl transition-all disabled:opacity-50"
                        >
                          {finishing ? '終了中...' : '終了する'}
                        </button>
                      </div>
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
