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
import { ApiError, apiErrorMessage, apiFetch } from "@/lib/api";
import { displayPlanText, formatLocalDate } from "@/lib/fitlog";
import { initialRecordFlowState, recordFlowReducer } from "./workoutFlow";

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
  plan_date: string;
  status: string;
  ai_status?: "applied" | "fallback" | "not_requested";
  plan: {
    workout_title: string;
    target: string;
    estimated_duration_min: number;
    coach_note: string;
    exercises: WorkoutPlanExercise[];
  };
}

interface SetRecommendation {
  next_action: "CONTINUE" | "STOP" | "ADJUST";
  recommendation: string;
  target_weight: number;
  target_reps: number;
  reason: string;
  record_template?: string;
  max_weight: number;
}

type SummaryCommentState = "idle" | "loading" | "ready" | "unavailable";

function newIdempotencyKey() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `fitlog_${Date.now().toString(36)}_${Math.random().toString(36).slice(2)}`;
}

function isAmbiguousSaveFailure(error: unknown) {
  if (!(error instanceof ApiError)) return true;
  return ![400, 401, 403, 404, 409, 422].includes(error.status);
}

export default function WorkoutPage() {
  const [starting, setStarting] = React.useState(false);
  const [finishing, setFinishing] = React.useState(false);
  const [recommendation, setRecommendation] = React.useState<SetRecommendation | null>(null);
  const [recordFlow, dispatchRecordFlow] = React.useReducer(recordFlowReducer, initialRecordFlowState);
  const saveState = recordFlow.save;
  const recommendationState = recordFlow.recommendation;
  const [summaryCommentState, setSummaryCommentState] = React.useState<SummaryCommentState>("idle");
  const [startError, setStartError] = React.useState<string | null>(null);
  const [manualStartAvailable, setManualStartAvailable] = React.useState(false);
  const [startNotice, setStartNotice] = React.useState<string | null>(null);
  const [saveError, setSaveError] = React.useState<string | null>(null);
  const [saveRetryLocked, setSaveRetryLocked] = React.useState(false);
  const [recommendationError, setRecommendationError] = React.useState<string | null>(null);
  const [finishError, setFinishError] = React.useState<string | null>(null);
  const [summaryCommentError, setSummaryCommentError] = React.useState<string | null>(null);
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
  const [showManualExerciseSelector, setShowManualExerciseSelector] = React.useState(false);
  const [targetSets, setTargetSets] = React.useState(3);
  const [editingTargetSets, setEditingTargetSets] = React.useState(false);
  const [tempTargetSets, setTempTargetSets] = React.useState(3);
  const [finishedSummary, setFinishedSummary] = React.useState<WorkoutSummary | null>(null);
  const [finishedWorkoutID, setFinishedWorkoutID] = React.useState<number | null>(null);
  const [recordedSetID, setRecordedSetID] = React.useState<number | null>(null);
  const startInFlightRef = React.useRef(false);
  const saveInFlightRef = React.useRef(false);
  const finishInFlightRef = React.useRef(false);
  const setRequestRef = React.useRef<{ signature: string; key: string; setID?: number } | null>(null);
  const activeSetIDRef = React.useRef<number | null>(null);
  const recommendationAbortRef = React.useRef<AbortController | null>(null);
  const recommendationRequestRef = React.useRef(0);
  const finishedWorkoutIDRef = React.useRef<number | null>(null);
  const summaryCommentAbortRef = React.useRef<AbortController | null>(null);
  const summaryCommentRequestRef = React.useRef(0);
  const nextSetByExerciseRef = React.useRef<Record<string, number>>({});
  const recordTransitionLocked = saveState === "saving" || saveRetryLocked;

  React.useEffect(() => () => {
    recommendationAbortRef.current?.abort();
    summaryCommentAbortRef.current?.abort();
  }, []);

  // 種目が変わったら目標セット数を取得
  React.useEffect(() => {
    const fetchTargetSets = async () => {
      if (workoutPlan) return;
      try {
        const data = await apiFetch<{ target_sets: number }>(`/api/exercises/${encodeURIComponent(currentExercise.id)}/settings`);
        setTargetSets(data.target_sets ?? 3);
        setTempTargetSets(data.target_sets ?? 3);
      } catch { /* デフォルト3セットのまま */ }
    };
    fetchTargetSets();
  }, [currentExercise.id, workoutPlan]);

  const saveTargetSets = async (value: number) => {
    if (saveInFlightRef.current || saveRetryLocked || finishInFlightRef.current) return;
    setTargetSets(value);
    setEditingTargetSets(false);
    try {
      await apiFetch<{ target_sets: number }>(`/api/exercises/${encodeURIComponent(currentExercise.id)}/settings`, {
        method: 'PUT',
        body: JSON.stringify({ target_sets: value })
      });
    } catch { /* 無視 */ }
  };

  const resetSetFlow = () => {
    recommendationAbortRef.current?.abort();
    recommendationRequestRef.current += 1;
    recommendationAbortRef.current = null;
    setRecommendation(null);
    dispatchRecordFlow({ type: "RESET" });
    setRecommendationError(null);
    setSaveError(null);
    setSaveRetryLocked(false);
    setRequestRef.current = null;
    activeSetIDRef.current = null;
    setRecordedSetID(null);
  };

  const activatePlanExercise = (plan: WorkoutPlanSession, index: number) => {
    if (saveInFlightRef.current || saveRetryLocked || finishInFlightRef.current) return;
    const ex = plan.plan.exercises[index];
    if (!ex) return;
    if (workoutPlan && index === currentExerciseIndex && ex.exercise_id === currentExercise.id) return;
    setCurrentExerciseIndex(index);
    setCurrentExercise({ id: ex.exercise_id, name: ex.name });
    setTargetSets(ex.planned_sets || 3);
    setTempTargetSets(ex.planned_sets || 3);
    setCurrentSet(nextSetByExerciseRef.current[ex.exercise_id] || 1);
    resetSetFlow();
    setFormData({
      weight: ex.target_weight ? String(ex.target_weight) : '',
      reps: ex.target_reps ? String(ex.target_reps) : '',
      feeling: ''
    });
  };

  const applyStartedWorkout = (data: WorkoutPlanSession) => {
    if (!data.plan.exercises.length) {
      throw new Error("開始できる種目がありません。");
    }
    setWorkoutPlan(data);
    setWorkoutStarted(true);
    setManualStartAvailable(false);
    nextSetByExerciseRef.current = {};
    setStartNotice(data.ai_status === "fallback"
      ? "AI調整は利用できませんでしたが、記録できるメニューで開始しました。"
      : null);
    activatePlanExercise(data, 0);
  };

  const startWorkout = async () => {
    if (startInFlightRef.current) return;
    startInFlightRef.current = true;
    setStarting(true);
    setStartError(null);
    setManualStartAvailable(false);
    setStartNotice(null);
    setFinishedSummary(null);
    setFinishedWorkoutID(null);
    finishedWorkoutIDRef.current = null;
    setSummaryCommentState("idle");
    setSummaryCommentError(null);
    try {
      const data = await apiFetch<WorkoutPlanSession>(`/api/workout-plans/${formatLocalDate(new Date())}/start`, {
        method: 'POST',
      });
      applyStartedWorkout(data);
    } catch (e) {
      console.error(e);
      setStartError(apiErrorMessage(e, '今日の計画作成に失敗しました。'));
      setManualStartAvailable(e instanceof ApiError && e.status === 404);
    } finally {
      startInFlightRef.current = false;
      setStarting(false);
    }
  };

  const startManualWorkout = async (exercise: { id: string; name: string }) => {
    if (startInFlightRef.current) return;
    startInFlightRef.current = true;
    setStarting(true);
    setStartError(null);
    setManualStartAvailable(false);
    setStartNotice(null);
    setShowManualExerciseSelector(false);
    try {
      const date = formatLocalDate(new Date());
      await apiFetch<WorkoutPlanSession>(`/api/workout-plans/${date}`, {
        method: "PUT",
        body: JSON.stringify({
          workout_title: "フリーワークアウト",
          target: "自分で選んだ種目",
          estimated_duration_min: 15,
          coach_note: "体調に合わせて無理のない重量で進めましょう。",
          exercises: [{
            exercise_id: exercise.id,
            name: exercise.name,
            planned_sets: 3,
            target_weight: 0,
            target_reps: 10,
          }],
        }),
      });
      const data = await apiFetch<WorkoutPlanSession>(`/api/workout-plans/${date}/start`, { method: "POST" });
      applyStartedWorkout(data);
    } catch (e) {
      console.error(e);
      setStartError(apiErrorMessage(e, "手動メニューでワークアウトを開始できませんでした。"));
    } finally {
      startInFlightRef.current = false;
      setStarting(false);
    }
  };

  const fetchRecommendation = async (workoutID: number, setID: number) => {
    recommendationAbortRef.current?.abort();
    const controller = new AbortController();
    const requestID = recommendationRequestRef.current + 1;
    recommendationRequestRef.current = requestID;
    recommendationAbortRef.current = controller;
    setRecommendation(null);
    dispatchRecordFlow({ type: "RECOMMENDATION_STARTED" });
    setRecommendationError(null);
    try {
      const data = await apiFetch<SetRecommendation>(`/api/workouts/${workoutID}/sets/${setID}/recommendation`, {
        method: "POST",
        signal: controller.signal,
      });
      if (controller.signal.aborted || recommendationRequestRef.current !== requestID || activeSetIDRef.current !== setID) return;
      setRecommendation(data);
      dispatchRecordFlow({ type: "RECOMMENDATION_SUCCEEDED" });
    } catch (e) {
      if (controller.signal.aborted || recommendationRequestRef.current !== requestID || activeSetIDRef.current !== setID) return;
      console.error(e);
      setRecommendationError(apiErrorMessage(e, "AI提案を取得できませんでした。"));
      dispatchRecordFlow({ type: "RECOMMENDATION_FAILED" });
    }
  };

  const saveCurrentSet = async () => {
    if (!formData.weight || !formData.reps || saveInFlightRef.current || finishInFlightRef.current || recordedSetID !== null) return;
    const workoutID = workoutPlan?.workout_id;
    if (!workoutID) {
      dispatchRecordFlow({ type: "SAVE_FAILED" });
      setSaveError("進行中のワークアウトが見つかりません。");
      return;
    }

    const weight = Number.parseFloat(formData.weight);
    const reps = Number.parseInt(formData.reps, 10);
    if (!Number.isFinite(weight) || weight < 0 || !Number.isInteger(reps) || reps <= 0) {
      dispatchRecordFlow({ type: "SAVE_FAILED" });
      setSaveRetryLocked(false);
      setSaveError("重量は0以上、回数は1以上の整数で入力してください。");
      return;
    }
    const setInput = {
      exercise_id: currentExercise.id,
      set_order: currentSet,
      weight,
      reps,
      feeling: formData.feeling,
    };
    const signature = JSON.stringify(setInput);
    if (!setRequestRef.current || setRequestRef.current.signature !== signature) {
      setRequestRef.current = { signature, key: newIdempotencyKey() };
    }

    saveInFlightRef.current = true;
    dispatchRecordFlow({ type: "SAVE_STARTED" });
    setSaveError(null);
    setRecommendation(null);
    setRecommendationError(null);
    try {
      const recorded = await apiFetch<{ id: number }>(`/api/workouts/${workoutID}/sets`, {
        method: "POST",
        headers: { "Idempotency-Key": setRequestRef.current.key },
        body: JSON.stringify(setInput),
      });
      setRequestRef.current.setID = recorded.id;
      activeSetIDRef.current = recorded.id;
      setRecordedSetID(recorded.id);
      nextSetByExerciseRef.current[currentExercise.id] = Math.max(
        nextSetByExerciseRef.current[currentExercise.id] || 1,
        currentSet + 1,
      );
      setSaveRetryLocked(false);
      dispatchRecordFlow({ type: "SAVE_SUCCEEDED" });
      void fetchRecommendation(workoutID, recorded.id);
    } catch (e) {
      console.error(e);
      dispatchRecordFlow({ type: "SAVE_FAILED" });
      setSaveRetryLocked(isAmbiguousSaveFailure(e));
      setSaveError(apiErrorMessage(e, "セットを保存できませんでした。入力内容はそのまま残っています。"));
    } finally {
      saveInFlightRef.current = false;
    }
  };

  const retryRecommendation = () => {
    if (!workoutPlan?.workout_id || !recordedSetID || recommendationState === "loading") return;
    void fetchRecommendation(workoutPlan.workout_id, recordedSetID);
  };

  const handleNextSet = () => {
    if (saveState !== "saved" || finishInFlightRef.current) return;
    const nextSet = currentSet + 1;
    nextSetByExerciseRef.current[currentExercise.id] = nextSet;
    setCurrentSet(nextSet);
    if (recommendation?.target_weight) {
      setFormData({
        weight: String(recommendation.target_weight),
        reps: recommendation.target_reps ? String(recommendation.target_reps) : formData.reps,
        feeling: ''
      });
    } else {
      setFormData({ ...formData, feeling: '' });
    }
    resetSetFlow();
  };

  const fetchSummaryComment = async (workoutID: number) => {
    summaryCommentAbortRef.current?.abort();
    const controller = new AbortController();
    const requestID = summaryCommentRequestRef.current + 1;
    summaryCommentRequestRef.current = requestID;
    summaryCommentAbortRef.current = controller;
    setSummaryCommentState("loading");
    setSummaryCommentError(null);
    try {
      const data = await apiFetch<{ comment: string; replayed: boolean }>(`/api/workouts/${workoutID}/summary-comment`, {
        method: "POST",
        signal: controller.signal,
      });
      if (controller.signal.aborted || summaryCommentRequestRef.current !== requestID || finishedWorkoutIDRef.current !== workoutID) return;
      setFinishedSummary(previous => previous ? { ...previous, ai_comment: data.comment } : previous);
      setSummaryCommentState("ready");
    } catch (e) {
      if (controller.signal.aborted || summaryCommentRequestRef.current !== requestID || finishedWorkoutIDRef.current !== workoutID) return;
      console.error(e);
      setSummaryCommentError(apiErrorMessage(e, "AI総評を取得できませんでした。"));
      setSummaryCommentState("unavailable");
    }
  };

  const finishWorkout = async () => {
    if (finishInFlightRef.current || saveInFlightRef.current || saveRetryLocked) return;
    finishInFlightRef.current = true;
    setFinishing(true);
    setFinishError(null);
    try {
      if (!workoutPlan?.workout_id) {
        setFinishError('進行中のワークアウトが見つかりません。');
        return;
      }
      const workoutID = workoutPlan.workout_id;
      const data = await apiFetch<{ summary: WorkoutSummary }>(`/api/workouts/${workoutID}/finish`, {
        method: 'POST',
      });
      recommendationAbortRef.current?.abort();
      activeSetIDRef.current = null;
      finishedWorkoutIDRef.current = workoutID;
      setFinishedWorkoutID(workoutID);
      setFinishedSummary(data.summary || null);
      if (data.summary?.ai_comment) {
        setSummaryCommentState("ready");
      } else {
        void fetchSummaryComment(workoutID);
      }
      setWorkoutStarted(false);
      setWorkoutPlan(null);
      setRecommendation(null);
      dispatchRecordFlow({ type: "RESET" });
      setCurrentSet(1);
      nextSetByExerciseRef.current = {};
      setCurrentExerciseIndex(0);
      setFormData({ weight: '', reps: '', feeling: '' });
      setRequestRef.current = null;
      setRecordedSetID(null);
    } catch (e) {
      console.error(e);
      setFinishError(apiErrorMessage(e, 'ワークアウト終了に失敗しました。'));
    } finally {
      finishInFlightRef.current = false;
      setFinishing(false);
    }
  };

  const handleNextExercise = () => {
    if (!workoutPlan || saveInFlightRef.current || saveRetryLocked || finishInFlightRef.current) return;
    const nextIndex = currentExerciseIndex + 1;
    if (nextIndex >= workoutPlan.plan.exercises.length) {
      void finishWorkout();
      return;
    }
    activatePlanExercise(workoutPlan, nextIndex);
  };

  const resetFinishedView = () => {
    summaryCommentAbortRef.current?.abort();
    summaryCommentRequestRef.current += 1;
    finishedWorkoutIDRef.current = null;
    setFinishedWorkoutID(null);
    setFinishedSummary(null);
    setSummaryCommentState("idle");
    setSummaryCommentError(null);
  };

  const activePlanExercise = workoutPlan?.plan.exercises.find(exercise => exercise.exercise_id === currentExercise.id);

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

          {summaryCommentState === "loading" && (
            <div className="mb-6 flex items-center gap-3 rounded-2xl border border-primary/20 bg-primary/5 p-4 text-sm text-white/65">
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
              <div>
                <p className="font-bold text-white/85">AIコーチが総評を作成中です</p>
                <p className="mt-0.5 text-xs text-white/40">記録と集計はすでに完了しています。</p>
              </div>
            </div>
          )}

          {summaryCommentState === "unavailable" && (
            <div className="mb-6 rounded-2xl border border-amber-400/25 bg-amber-400/10 p-4">
              <div className="flex items-start gap-3">
                <BrainCircuit className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-300" />
                <div>
                  <p className="text-sm font-bold text-amber-200">AI総評のみ取得できませんでした</p>
                  <p className="mt-1 text-xs text-amber-100/60">{summaryCommentError}</p>
                  <button
                    type="button"
                    onClick={() => finishedWorkoutID && void fetchSummaryComment(finishedWorkoutID)}
                    disabled={!finishedWorkoutID}
                    className="mt-3 flex items-center gap-1.5 text-xs font-bold text-amber-200 underline transition-colors hover:text-amber-100 disabled:opacity-40"
                  >
                    <RefreshCw className="h-3.5 w-3.5" /> AI総評を再試行
                  </button>
                </div>
              </div>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <button
              onClick={resetFinishedView}
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

        {showManualExerciseSelector && (
          <ExerciseSelectorModal
            onClose={() => setShowManualExerciseSelector(false)}
            onSelect={(exercise) => void startManualWorkout(exercise)}
          />
        )}

        <div className="z-10 text-center max-w-sm w-full mx-auto">
          <div className="w-24 h-24 bg-primary/20 rounded-full flex items-center justify-center mx-auto mb-8 relative">
            <div className="absolute inset-0 bg-primary/20 rounded-full animate-ping" />
            <Play className="w-10 h-10 text-primary ml-2" />
          </div>
          <h1 className="text-3xl font-bold mb-4">今日の筋トレを始めますか？</h1>
          <p className="text-white/50 mb-10">今日のメニューを開始します。AIが使えないときも記録は続けられます。</p>
          <button 
            onClick={startWorkout}
            disabled={starting}
            className="w-full py-5 bg-primary text-black font-black rounded-2xl shadow-[0_0_30px_rgba(255,170,0,0.5)] hover:scale-[1.02] active:scale-[0.98] transition-all duration-300 text-xl tracking-wider group relative overflow-hidden disabled:opacity-50 disabled:pointer-events-none"
          >
            <div className="absolute inset-0 -translate-x-full group-hover:animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-transparent via-white/30 to-transparent skew-x-12" />
            <span className="relative z-10 flex items-center justify-center gap-2">
              {starting ? <Loader2 className="w-6 h-6 animate-spin" /> : <>ワークアウトを開始 <Sparkles className="w-5 h-5" /></>}
            </span>
          </button>
          {manualStartAvailable && (
            <div className="mt-3">
              <button
                type="button"
                onClick={() => setShowManualExerciseSelector(true)}
                disabled={starting}
                className="w-full rounded-2xl border border-white/10 bg-white/5 py-4 text-sm font-bold text-white/70 transition-colors hover:bg-white/10 hover:text-white disabled:pointer-events-none disabled:opacity-40"
              >
                自分で種目を選んで始める
              </button>
              <p className="mt-2 text-[11px] text-white/30">月間プランがないため、1種目のフリーワークアウトを作成できます。</p>
            </div>
          )}
          {startError && (
            <div className="mt-5 p-4 bg-red-500/10 border border-red-500/30 rounded-2xl text-left">
              <p className="text-red-300 font-bold text-sm">エラーが発生しました</p>
              <p className="text-red-200/70 text-xs mt-1 whitespace-pre-line">{startError}</p>
              <button
                onClick={startWorkout}
                disabled={starting}
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
              if (recordTransitionLocked || finishing) return;
              const nextExercise = typeof newExercise === "string"
                ? { id: `custom_${Date.now()}`, name: newExercise }
                : { id: newExercise.id || `custom_${Date.now()}`, name: newExercise.name || currentExercise.name };
              setCurrentExercise({ id: nextExercise.id, name: nextExercise.name });
              setCurrentSet(nextSetByExerciseRef.current[nextExercise.id] || 1);
              resetSetFlow();
              setShowAltModal(false);
            }}
          />
        )}
      </AnimatePresence>

      {showExerciseSelector && (
        <ExerciseSelectorModal
          onClose={() => setShowExerciseSelector(false)}
          onSelect={(ex) => {
            if (recordTransitionLocked || finishing) return;
            setCurrentExercise(ex);
            setCurrentSet(nextSetByExerciseRef.current[ex.id] || 1);
            resetSetFlow();
            setShowExerciseSelector(false);
          }}
        />
      )}

      <main className="max-w-3xl mx-auto relative z-10">
        <header className="mb-8 flex flex-col gap-4 sm:flex-row sm:justify-between sm:items-center">
            <div>
              <h1 className="text-2xl font-bold tracking-tight">ワークアウト中</h1>
              <p className="text-primary font-medium text-sm flex items-center gap-2">
                 <BrainCircuit className="w-4 h-4" /> 記録を優先・AIコーチも利用可能
              </p>
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={finishWorkout}
                disabled={finishing || recordTransitionLocked}
                className="px-4 py-2 bg-white/5 hover:bg-accent/20 border border-white/10 hover:border-accent/30 rounded-xl text-xs font-bold text-white/60 hover:text-accent transition-colors disabled:opacity-50"
              >
                {finishing ? "終了中..." : "終了"}
              </button>
            </div>
        </header>

        {startNotice && (
          <div className="mb-5 flex items-start gap-3 rounded-2xl border border-amber-400/25 bg-amber-400/10 p-4 text-sm text-amber-100/80">
            <BrainCircuit className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-300" />
            <p>{startNotice}</p>
          </div>
        )}

        {finishError && (
          <div className="mb-5 rounded-2xl border border-red-500/30 bg-red-500/10 p-4">
            <p className="text-sm font-bold text-red-300">ワークアウトを終了できませんでした</p>
            <p className="mt-1 text-xs text-red-200/65">{finishError}</p>
          </div>
        )}

        <section className="glass rounded-[32px] p-6 lg:p-8 border-primary/20 bg-primary/5 overflow-hidden relative shadow-2xl shadow-primary/5">
          <div className="flex justify-between items-center mb-8 border-b border-white/5 pb-6">
            <div>
              <h2 className="text-2xl font-bold tracking-wider mb-2">{currentExercise.name}</h2>
              <div className="flex gap-2">
                <button 
                  onClick={() => setShowExerciseSelector(true)}
                  disabled={recordTransitionLocked || finishing}
                  className="text-[10px] font-bold text-primary hover:text-primary/80 flex items-center gap-1 transition-colors bg-primary/10 hover:bg-primary/20 px-3 py-1.5 rounded-lg border border-primary/20 disabled:pointer-events-none disabled:opacity-40"
                >
                  <Search className="w-3 h-3" /> 種目を検索して変更
                </button>
                <button 
                  onClick={() => setShowAltModal(true)}
                  disabled={recordTransitionLocked || finishing}
                  className="text-[10px] font-bold text-white/50 hover:text-primary flex items-center gap-1 transition-colors bg-white/5 hover:bg-white/10 px-3 py-1.5 rounded-lg border border-white/10 disabled:pointer-events-none disabled:opacity-40"
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
                    i < currentSet - 1 + (saveState === "saved" ? 1 : 0) ? 'bg-primary' : 'bg-white/10'
                  }`}
                />
              ))}
            </div>
            {/* 目標セット数の編集 */}
            {editingTargetSets ? (
              <div className="flex items-center gap-1 mt-1">
                <button onClick={() => saveTargetSets(Math.max(1, tempTargetSets - 1))} disabled={finishing || recordTransitionLocked} className="w-6 h-6 bg-white/10 rounded-lg text-white font-bold hover:bg-white/20 transition-colors flex items-center justify-center text-xs disabled:opacity-40">−</button>
                <span className="text-xs text-white/70 w-10 text-center font-bold">{tempTargetSets} セット</span>
                <button onClick={() => saveTargetSets(Math.min(20, tempTargetSets + 1))} disabled={finishing || recordTransitionLocked} className="w-6 h-6 bg-white/10 rounded-lg text-white font-bold hover:bg-white/20 transition-colors flex items-center justify-center text-xs disabled:opacity-40">＋</button>
                <button onClick={() => setEditingTargetSets(false)} disabled={finishing || recordTransitionLocked} className="text-[10px] text-white/40 ml-1 hover:text-white/60 disabled:opacity-40">✕</button>
              </div>
            ) : (
              <button
                onClick={() => { setTempTargetSets(targetSets); setEditingTargetSets(true); }}
                disabled={finishing || recordTransitionLocked}
                className="text-[10px] text-white/30 hover:text-white/60 transition-colors mt-0.5 disabled:opacity-40"
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
                    disabled={recordTransitionLocked || finishing || (idx === currentExerciseIndex && ex.exercise_id === currentExercise.id)}
                    className={`text-left p-3 rounded-xl border transition-all ${
                      idx === currentExerciseIndex
                        ? 'bg-primary/15 border-primary/40 text-white'
                        : 'bg-white/5 border-white/5 text-white/55 hover:bg-white/10 hover:text-white'
                    } disabled:pointer-events-none disabled:opacity-40`}
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
                    disabled={recordTransitionLocked || saveState === "saved" || finishing}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-2xl font-bold disabled:opacity-60"
                    placeholder="80"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-[10px] font-black text-white/40 ml-1">回数</label>
                  <input 
                    type="number" 
                    value={formData.reps}
                    onChange={(e) => setFormData({...formData, reps: e.target.value})}
                    disabled={recordTransitionLocked || saveState === "saved" || finishing}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-secondary/50 text-2xl font-bold disabled:opacity-60"
                    placeholder="10"
                  />
                </div>
              </div>
              <div className="space-y-2">
                <label className="text-[10px] font-black text-white/40 ml-1">セット後の感想</label>
                <textarea 
                  value={formData.feeling}
                  onChange={(e) => setFormData({...formData, feeling: e.target.value})}
                  disabled={recordTransitionLocked || saveState === "saved" || finishing}
                  className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-accent/50 text-sm min-h-[100px] disabled:opacity-60"
                  placeholder="例: きつかった、まだ余裕がある、肩に違和感..."
                />
              </div>

              {saveState === "saved" && (
                <div className="flex items-center gap-2 rounded-2xl border border-green-500/25 bg-green-500/10 px-4 py-3 text-xs font-bold text-green-300">
                  <CheckCircle2 className="h-4 w-4 flex-shrink-0" />
                  セットは保存済みです。AIの状態にかかわらず次へ進めます。
                </div>
              )}

              {saveState !== "saved" && (
                <button 
                  onClick={saveCurrentSet}
                  disabled={saveState === "saving" || finishing || !formData.weight || !formData.reps}
                  className="w-full py-5 mt-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-30 disabled:pointer-events-none tracking-widest flex items-center justify-center gap-2 shadow-[0_0_20px_rgba(255,170,0,0.3)]"
                >
                  {saveState === "saving"
                    ? <><Loader2 className="w-6 h-6 animate-spin" /> 保存中...</>
                    : saveState === "failed" ? "セットの保存を再試行" : "セットを保存"}
                </button>
              )}

              {saveState === "failed" && saveError && (
                <div className="mt-4 p-4 bg-red-500/10 border border-red-500/30 rounded-2xl flex items-start gap-3">
                  <Flame className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-red-400 font-bold text-sm">セットを保存できませんでした</p>
                    <p className="text-red-300/70 text-xs mt-1 whitespace-pre-line">{saveError}</p>
                    <p className="mt-2 text-[11px] text-red-200/50">
                      {saveRetryLocked
                        ? "保存済みか判定できないため入力と移動を固定し、同じ識別子で安全に再送します。"
                        : "入力内容を確認してから、もう一度保存してください。"}
                    </p>
                  </div>
                </div>
              )}

              {saveState === "saved" && recommendationState === "loading" && (
                <div className="rounded-2xl border border-primary/20 bg-primary/5 p-4">
                  <div className="flex items-center gap-3">
                    <Loader2 className="h-5 w-5 animate-spin text-primary" />
                    <div>
                      <p className="text-sm font-bold text-white/85">AI提案を取得中</p>
                      <p className="mt-0.5 text-xs text-white/40">待たずに同じ重量・回数で次へ進めます。</p>
                    </div>
                  </div>
                </div>
              )}

              {saveState === "saved" && recommendationState === "unavailable" && (
                <div className="rounded-2xl border border-amber-400/25 bg-amber-400/10 p-4">
                  <div className="flex items-start gap-3">
                    <BrainCircuit className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-300" />
                    <div>
                      <p className="text-sm font-bold text-amber-200">AI提案のみ取得できませんでした</p>
                      <p className="mt-1 text-xs text-amber-100/60">{recommendationError}</p>
                      <button
                        type="button"
                        onClick={retryRecommendation}
                        className="mt-3 flex items-center gap-1.5 text-xs font-bold text-amber-200 underline transition-colors hover:text-amber-100"
                      >
                        <RefreshCw className="h-3.5 w-3.5" /> AI提案だけ再試行
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {saveState === "saved" && recommendationState !== "ready" && (
                <div className="space-y-3 rounded-2xl border border-white/10 bg-black/25 p-4">
                  <div className="grid grid-cols-1 gap-2 text-center sm:grid-cols-3">
                    <div className="rounded-xl bg-white/5 p-3">
                      <p className="text-[10px] font-bold text-white/35">直前のセット</p>
                      <p className="mt-1 text-sm font-bold">{formData.weight}kg × {formData.reps}回</p>
                    </div>
                    <div className="rounded-xl bg-white/5 p-3">
                      <p className="text-[10px] font-bold text-white/35">計画上の目標</p>
                      <p className="mt-1 text-sm font-bold">
                        {activePlanExercise
                          ? `${activePlanExercise.target_weight ? `${activePlanExercise.target_weight}kg × ` : ""}${activePlanExercise.target_reps}回`
                          : "自分で調整"}
                      </p>
                    </div>
                    <div className="rounded-xl bg-white/5 p-3">
                      <p className="text-[10px] font-bold text-white/35">過去の最大重量</p>
                      <p className="mt-1 text-sm font-bold">
                        {activePlanExercise?.last_max_weight
                          ? `${activePlanExercise.last_max_weight}kg`
                          : "記録なし"}
                      </p>
                    </div>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <button
                      type="button"
                      onClick={handleNextSet}
                      disabled={finishing}
                      className="w-full rounded-2xl bg-white/10 py-4 font-bold text-white transition-colors hover:bg-white/20 disabled:opacity-40"
                    >
                      {recommendationState === "loading" ? "AIを待たず次セットへ" : "同じ内容で次へ"}
                    </button>
                    <button
                      type="button"
                      onClick={handleNextExercise}
                      disabled={finishing}
                      className="w-full rounded-2xl bg-primary/20 py-4 font-bold text-primary transition-colors hover:bg-primary/30 disabled:opacity-40"
                    >
                      {currentExerciseIndex + 1 >= (workoutPlan?.plan.exercises.length || 1) ? 'ワークアウト終了' : '次の種目へ'}
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* AI Result Area - Only shows when recommendation exists */}
            {recommendationState === "ready" && recommendation && (
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
                          disabled={finishing}
                          className="w-full py-4 bg-white/10 hover:bg-white/20 text-white font-bold rounded-2xl transition-all disabled:opacity-40"
                        >
                          次のセットへ
                        </button>
                        <button 
                          onClick={handleNextExercise}
                          disabled={finishing}
                          className="w-full py-4 bg-primary/20 hover:bg-primary/30 text-primary font-bold rounded-2xl transition-all disabled:opacity-40"
                        >
                          {currentExerciseIndex + 1 >= (workoutPlan?.plan.exercises.length || 1) ? 'ワークアウト終了' : '次の種目へ'}
                        </button>
                      </div>
                    ) : (
                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                        <button 
                          onClick={handleNextSet}
                          disabled={finishing}
                          className="w-full py-4 bg-white/10 hover:bg-white/20 text-white font-bold rounded-2xl transition-all disabled:opacity-40"
                        >
                          もう1セットだけ続ける
                        </button>
                        <button 
                          onClick={handleNextExercise}
                          disabled={finishing}
                          className="w-full py-4 bg-primary/20 hover:bg-primary/30 text-primary font-bold rounded-2xl transition-all disabled:opacity-40"
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
