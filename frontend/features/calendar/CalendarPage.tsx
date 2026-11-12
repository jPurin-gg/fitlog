"use client";

import React, { useEffect, useState } from "react";
import { ArrowLeft, Calendar as CalendarIcon, ChevronLeft, ChevronRight, Dumbbell, Sparkles, Check, Plus, Trash2, Save, Loader2, X, ExternalLink, RefreshCw } from "lucide-react";
import Link from "next/link";
import { motion } from "framer-motion";
import { ExerciseSelectorModal } from "@/features/exercises/components/ExerciseSelectorModal";
import { useBodyScrollLock } from "@/hooks/useBodyScrollLock";
import { ApiError, apiErrorMessage, apiFetch } from "@/lib/api";
import { displayPlanText, formatLocalDate, formatPlanMonth } from "@/lib/fitlog";
import type { CalendarPlan, CalendarPlanExercise, CalendarWorkout, CalendarWorkoutSet, DayRoutine, MonthlyPlan, PlannedWorkoutDay, WorkedOutDay } from "./types";

function dateFromPlanMonth(planMonth: string) {
  const [year, month] = planMonth.split("-").map(Number);
  return new Date(year, (month || 1) - 1, 1);
}

function isFutureDateKey(dateKey: string) {
  const [year, month, day] = dateKey.split("-").map(Number);
  const target = new Date(year, (month || 1) - 1, day || 1);
  const today = new Date();
  const todayStart = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  return target > todayStart;
}

const WEEKDAY_OPTIONS = [
  { value: 0, label: "日曜" },
  { value: 1, label: "月曜" },
  { value: 2, label: "火曜" },
  { value: 3, label: "水曜" },
  { value: 4, label: "木曜" },
  { value: 5, label: "金曜" },
  { value: 6, label: "土曜" },
];

function weekdayLabels(days?: number[]) {
  if (!days || days.length === 0) return "";
  return WEEKDAY_OPTIONS
    .filter(day => days.includes(day.value))
    .map(day => day.label)
    .join("・");
}

type LoadStatus = "loading" | "ready" | "error";

interface CalendarResponse {
  worked_out_dates?: number[];
  worked_out_days?: WorkedOutDay[];
  planned_days?: PlannedWorkoutDay[];
}

interface MonthCalendar {
  workedOutDates: number[];
  workedOutDays: WorkedOutDay[];
  plannedDays: PlannedWorkoutDay[];
}

// 取得結果は対象の月（YYYY-MM）と一緒に持ち、表示中の月と一致するものだけを描画に使う。
interface MonthCalendarLoad {
  month: string;
  status: LoadStatus;
  data: MonthCalendar;
}

interface MonthPlanLoad {
  month: string;
  status: LoadStatus;
  plan: MonthlyPlan | null;
}

const EMPTY_MONTH_CALENDAR: MonthCalendar = { workedOutDates: [], workedOutDays: [], plannedDays: [] };

const UNSAVED_CHANGES_MESSAGE = "保存していない変更があります。破棄して閉じますか？";

// 編集モーダルの未保存判定に使う。読み込み時と現在の入力を同じ形にして比べる。
// セット順は保存時に並び順から振り直すので、比較からは外す。
function workoutSnapshot(title: string, sets: CalendarWorkoutSet[]) {
  return JSON.stringify({ title, sets: sets.map(set => ({ ...set, set_order: 0 })) });
}

function planSnapshot(title: string, target: string, duration: number, coachNote: string, exercises: CalendarPlanExercise[]) {
  return JSON.stringify({ title, target, duration, coachNote, exercises });
}

export default function CalendarPage() {
  const [currentDate, setCurrentDate] = useState(new Date());
  const [planHistory, setPlanHistory] = useState<MonthlyPlan[]>([]);
  const [planLoad, setPlanLoad] = useState<MonthPlanLoad>({ month: "", status: "loading", plan: null });
  const [calendarLoad, setCalendarLoad] = useState<MonthCalendarLoad>({ month: "", status: "loading", data: EMPTY_MONTH_CALENDAR });
  const [planReloadKey, setPlanReloadKey] = useState(0);
  const [calendarReloadKey, setCalendarReloadKey] = useState(0);
  const [editingDate, setEditingDate] = useState<string | null>(null);
  const selectedPlanMonth = formatPlanMonth(currentDate);
  const year = currentDate.getFullYear();
  const month = currentDate.getMonth();

  useEffect(() => {
    apiFetch<MonthlyPlan[]>("/api/monthly-plans")
      .then(data => setPlanHistory(Array.isArray(data) ? data : []))
      .catch(err => console.error("Failed to fetch monthly plan history:", err));
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    // 月が変わったら前の月の結果を捨てる。同じ月の再取得では、成否にかかわらずその月の結果を残す。
    setPlanLoad(prev => prev.month === selectedPlanMonth
      ? { ...prev, status: "loading" }
      : { month: selectedPlanMonth, status: "loading", plan: null });
    apiFetch<MonthlyPlan>(`/api/monthly-plans/${selectedPlanMonth}`, { signal: controller.signal })
      .then(data => {
        if (controller.signal.aborted) return;
        setPlanLoad({ month: selectedPlanMonth, status: "ready", plan: data });
      })
      .catch(err => {
        if (controller.signal.aborted) return;
        if (err instanceof ApiError && err.status === 404) {
          setPlanLoad({ month: selectedPlanMonth, status: "ready", plan: null });
          return;
        }
        console.error("Failed to fetch monthly plan:", err);
        setPlanLoad(prev => prev.month === selectedPlanMonth
          ? { ...prev, status: "error" }
          : { month: selectedPlanMonth, status: "error", plan: null });
      });
    return () => controller.abort();
  }, [selectedPlanMonth, planReloadKey]);

  useEffect(() => {
    const controller = new AbortController();
    setCalendarLoad(prev => prev.month === selectedPlanMonth
      ? { ...prev, status: "loading" }
      : { month: selectedPlanMonth, status: "loading", data: EMPTY_MONTH_CALENDAR });
    apiFetch<CalendarResponse>(`/api/calendar?year=${year}&month=${month + 1}`, { signal: controller.signal })
      .then(data => {
        if (controller.signal.aborted) return;
        setCalendarLoad({
          month: selectedPlanMonth,
          status: "ready",
          data: {
            workedOutDates: data?.worked_out_dates ?? [],
            workedOutDays: data?.worked_out_days ?? [],
            plannedDays: data?.planned_days ?? [],
          },
        });
      })
      .catch(err => {
        if (controller.signal.aborted) return;
        console.error("Failed to fetch calendar data:", err);
        setCalendarLoad(prev => prev.month === selectedPlanMonth
          ? { ...prev, status: "error" }
          : { month: selectedPlanMonth, status: "error", data: EMPTY_MONTH_CALENDAR });
      });
    return () => controller.abort();
  }, [year, month, selectedPlanMonth, calendarReloadKey]);

  // 月を切り替えた直後は、取得が終わるまで前の月の結果を使わない。
  const planView: MonthPlanLoad = planLoad.month === selectedPlanMonth
    ? planLoad
    : { month: selectedPlanMonth, status: "loading", plan: null };
  const calendarView: MonthCalendarLoad = calendarLoad.month === selectedPlanMonth
    ? calendarLoad
    : { month: selectedPlanMonth, status: "loading", data: EMPTY_MONTH_CALENDAR };
  const plan = planView.plan;
  const plannedWeekDays = plan ? plan.recommended_days || [1, 3, 5] : [];
  const { workedOutDates, workedOutDays, plannedDays } = calendarView.data;

  const isSelectedCurrentMonth = selectedPlanMonth === formatPlanMonth(new Date());

  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const firstDayOfMonth = new Date(year, month, 1).getDay();

  const days: (number | null)[] = [];
  for (let i = 0; i < firstDayOfMonth; i++) {
    days.push(null);
  }
  for (let i = 1; i <= daysInMonth; i++) {
    days.push(i);
  }

  const today = new Date();
  const todayStart = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const isCurrentMonth = today.getFullYear() === year && today.getMonth() === month;

  const getDayStatus = (dateNum: number | null) => {
    if (!dateNum) return { isToday: false, isWorkedOut: false, isPlanned: false, isMissed: false, type: "", workoutId: null as number | null };
    const dateObj = new Date(year, month, dateNum);
    const isToday = isCurrentMonth && dateNum === today.getDate();
    const isWorkedOut = workedOutDates.includes(dateNum);
    
    const plannedDay = plannedDays.find(d => d.date === dateNum);
    const dow = dateObj.getDay();
    const plannedIdx = plannedWeekDays.indexOf(dow);
    const isPlanned = plannedIdx !== -1 || Boolean(plannedDay);

    let plannedTarget = "";
    if (plannedDay?.target) {
      plannedTarget = plannedDay.target;
    } else if (isPlanned && plan?.weekly_routine?.[plannedIdx]) {
      plannedTarget = plan.weekly_routine[plannedIdx].target;
      // カレンダー内に収まるよう、代表的な分割名は短く表示する。
      if (plannedTarget.includes("Push")) plannedTarget = "押す日";
      if (plannedTarget.includes("Pull")) plannedTarget = "引く日";
      if (plannedTarget.includes("Legs")) plannedTarget = "脚の日";
    }

    const workedOutDay = workedOutDays.find(d => d.date === dateNum);
    const type = workedOutDay ? displayPlanText(workedOutDay.type) : "";
    const workoutId = workedOutDay?.workout_id || null;

    // 「予定」は表示中の月に関係なく今日以降だけ。過ぎた日の未実施は「未実施」として分ける。
    const isUpcoming = dateObj >= todayStart;

    return {
      isToday,
      isWorkedOut,
      type,
      workoutId,
      plannedTarget,
      isPlanned: isPlanned && !isWorkedOut && isUpcoming,
      isMissed: isPlanned && !isWorkedOut && !isUpcoming,
    };
  };

  const weekDayNames = ["日", "月", "火", "水", "木", "金", "土"];
  const moveMonth = (offset: number) => {
    setCurrentDate(prev => new Date(prev.getFullYear(), prev.getMonth() + offset, 1));
  };
  const openDayEditor = (day: number) => {
    setEditingDate(`${year}-${String(month + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`);
  };

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 font-sans selection:bg-primary/30">
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="max-w-5xl mx-auto relative z-10">
        <header className="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between mb-10">
          <div className="flex items-center gap-4">
            <Link href="/" className="p-3 glass rounded-2xl hover:bg-white/10 transition-colors">
              <ArrowLeft className="w-5 h-5" />
            </Link>
            <div>
              <h1 className="text-3xl font-bold tracking-tight">カレンダー</h1>
              <p className="text-white/50">月間プランと実施記録</p>
            </div>
          </div>
          <button
            onClick={() => setCurrentDate(new Date())}
            className="self-start sm:self-auto px-4 py-2 bg-white/5 border border-white/10 rounded-xl text-xs font-bold text-white/60 hover:text-white hover:bg-white/10 transition-colors"
          >
            今月へ戻る
          </button>
        </header>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2">
            <div className="glass rounded-[32px] p-6 lg:p-8">
              <div className="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between mb-4">
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => moveMonth(-1)}
                    className="w-9 h-9 rounded-xl bg-white/5 border border-white/10 flex items-center justify-center text-white/60 hover:text-white hover:bg-white/10 transition-colors"
                    aria-label="前の月"
                  >
                    <ChevronLeft className="w-5 h-5" />
                  </button>
                  <h2 className="text-xl md:text-2xl font-black tracking-widest text-primary flex items-center gap-3">
                    <CalendarIcon className="w-5 h-5 md:w-6 md:h-6" />
                    {currentDate.toLocaleDateString('ja-JP', { year: 'numeric', month: 'long' })}
                  </h2>
                  <button
                    onClick={() => moveMonth(1)}
                    className="w-9 h-9 rounded-xl bg-white/5 border border-white/10 flex items-center justify-center text-white/60 hover:text-white hover:bg-white/10 transition-colors"
                    aria-label="次の月"
                  >
                    <ChevronRight className="w-5 h-5" />
                  </button>
                </div>
                
                <div className="flex flex-wrap gap-4 text-[10px] md:text-xs font-bold text-white/60">
                  <div className="flex items-center gap-1 md:gap-2">
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full bg-green-500 shadow-[0_0_10px_rgba(34,197,94,0.5)]"></div>
                    実施済み
                  </div>
                  <div className="flex items-center gap-1 md:gap-2">
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full bg-primary shadow-[0_0_10px_rgba(255,170,0,0.5)]"></div>
                    予定
                  </div>
                  <div className="flex items-center gap-1 md:gap-2">
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full border border-dashed border-white/60"></div>
                    未実施
                  </div>
                  <div className="flex items-center gap-1 md:gap-2">
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full border-2 border-white"></div>
                    今日
                  </div>
                </div>
              </div>

              <div className="mb-4 min-h-5">
                {calendarView.status === "loading" && (
                  <p role="status" className="flex items-center gap-2 text-xs text-white/50">
                    <Loader2 className="w-4 h-4 animate-spin" /> 実施記録と予定を読み込み中...
                  </p>
                )}
                {calendarView.status === "error" && (
                  <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200">
                    <span>実施記録と予定を読み込めませんでした。</span>
                    <button
                      type="button"
                      onClick={() => setCalendarReloadKey(key => key + 1)}
                      className="px-3 py-1.5 rounded-xl bg-white/10 text-xs font-bold text-white hover:bg-white/20 transition-colors flex items-center gap-1.5"
                    >
                      <RefreshCw className="w-3.5 h-3.5" /> カレンダーを再読み込み
                    </button>
                  </div>
                )}
              </div>

              <div className="grid grid-cols-7 gap-2 lg:gap-4 mb-4">
                {weekDayNames.map(day => (
                  <div key={day} className="text-center text-[10px] font-black text-white/30 tracking-widest">
                    {day}
                  </div>
                ))}
              </div>

              <div className="grid grid-cols-7 gap-2 lg:gap-4">
                {days.map((day, idx) => {
                  if (!day) return <div key={`empty-${idx}`} className="p-2" />;
                  
                  const status = getDayStatus(day);
                  
                  let bgClass = "bg-white/5 border-white/5";
                  let textClass = "text-white/50";
                  let statusLabel = "";

                  if (status.isWorkedOut) {
                    bgClass = "bg-green-500/20 border-green-500/30 ring-1 ring-green-500/50";
                    textClass = "text-green-400 font-bold";
                    statusLabel = "実施済み";
                  } else if (status.isPlanned) {
                    bgClass = "bg-primary/20 border-primary/30 ring-1 ring-primary/50 relative overflow-hidden";
                    textClass = "text-primary font-bold";
                    statusLabel = "予定";
                  } else if (status.isMissed) {
                    bgClass = "bg-white/5 border-dashed border-white/25";
                    textClass = "text-white/40";
                    statusLabel = "未実施";
                  }

                  if (status.isToday) {
                    bgClass += " ring-2 ring-white ring-offset-2 ring-offset-[#0a0a0a]";
                    textClass = "text-white font-black";
                  }

                  const dayContent = (
                    <motion.div
                      key={day}
                      initial={{ opacity: 0, scale: 0.9 }}
                      animate={{ opacity: 1, scale: 1 }}
                      transition={{ delay: idx * 0.01 }}
                      className={`relative aspect-square rounded-2xl border flex flex-col items-center justify-center transition-all ${
                        "hover:scale-105 cursor-pointer group"
                      } ${bgClass}`}
                    >
                      {status.isPlanned && !status.isWorkedOut && (
                        <div className="absolute inset-0 bg-gradient-to-tr from-primary/10 to-transparent opacity-50" />
                      )}
                      
                      <span className={`text-sm md:text-xl lg:text-2xl mb-1 ${textClass}`}>{day}</span>
                      
                      <div className="h-4 flex items-center justify-center">
                        {status.isWorkedOut ? (
                          <div className="flex flex-col items-center">
                            <span className="text-[8px] md:text-[10px] font-black text-green-300 leading-none text-center whitespace-normal px-1 w-full line-clamp-2">
                              {status.type}
                            </span>
                          </div>
                        ) : (
                          <>
                            {status.isPlanned && !status.isToday && (
                              <div className="flex flex-col items-center">
                                <span className="text-[7px] md:text-[9px] font-bold text-primary opacity-80 leading-tight text-center whitespace-normal px-0.5 w-full line-clamp-2">
                                  {status.plannedTarget || <Dumbbell className="w-3 h-3 mx-auto" />}
                                </span>
                              </div>
                            )}
                            {status.isToday && status.isPlanned && (
                              <div className="flex flex-col items-center animate-pulse">
                                <span className="text-[8px] md:text-[10px] font-black text-white leading-tight text-center whitespace-normal px-0.5 w-full line-clamp-2">
                                  {status.plannedTarget || <Dumbbell className="w-4 h-4 mx-auto" />}
                                </span>
                              </div>
                            )}
                            {status.isMissed && (
                              <span className="text-[7px] md:text-[9px] font-bold text-white/40 leading-tight text-center">
                                未実施
                              </span>
                            )}
                          </>
                        )}
                      </div>
                    </motion.div>
                  );

                  return (
                    <button
                      key={day}
                      type="button"
                      onClick={() => openDayEditor(day)}
                      aria-label={`${selectedPlanMonth}-${String(day).padStart(2, "0")} の記録を編集${statusLabel ? `（${statusLabel}）` : ""}`}
                      className="block text-left"
                    >
                      {dayContent}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          <div className="space-y-6">
            <div className="glass rounded-[32px] p-6 lg:p-8">
              <div className="flex items-center justify-between gap-3 mb-5">
                <div>
                  <h2 className="text-lg font-bold tracking-tight">プラン履歴</h2>
                  <p className="text-xs text-white/40">過去月のプランを閲覧</p>
                </div>
                <span className="text-[10px] font-black text-white/30 bg-white/5 px-2 py-1 rounded-md">
                  {planHistory.length}
                </span>
              </div>

              {planHistory.length > 0 ? (
                <div className="space-y-2 max-h-72 overflow-y-auto pr-1">
                  {planHistory.map(historyPlan => {
                    const active = historyPlan.plan_month === selectedPlanMonth;
                    return (
                      <button
                        key={historyPlan.id || historyPlan.plan_month}
                        onClick={() => historyPlan.plan_month && setCurrentDate(dateFromPlanMonth(historyPlan.plan_month))}
                        className={`w-full text-left p-3 rounded-2xl border transition-all ${
                          active
                            ? "bg-primary/20 border-primary/40 text-white"
                            : "bg-white/5 border-white/5 text-white/60 hover:bg-white/10 hover:text-white"
                        }`}
                      >
                        <div className="flex justify-between items-start gap-3">
                          <div>
                            <div className="text-xs font-black text-primary mb-1">{historyPlan.plan_month}</div>
                            <div className="text-sm font-bold leading-tight">{displayPlanText(historyPlan.plan_name)}</div>
                          </div>
                          {historyPlan.plan_month === formatPlanMonth(new Date()) && (
                            <span className="text-[9px] font-black text-black bg-primary px-2 py-0.5 rounded-full">今月</span>
                          )}
                        </div>
                      </button>
                    );
                  })}
                </div>
              ) : (
                <p className="text-white/40 text-sm leading-relaxed">
                  まだ保存済みの月間プランがありません。ホームで今月のプランを作成すると、ここに履歴として残ります。
                </p>
              )}
            </div>

            <div className="glass rounded-[32px] p-6 lg:p-8 border-l-4 border-primary">
              <div className="flex items-center gap-2 text-primary font-bold mb-4 text-sm tracking-widest">
                <Sparkles className="w-5 h-5" /> {isSelectedCurrentMonth ? "今月のプラン" : "過去のプラン"}
              </div>
              {plan ? (
                <div>
                  <h3 className="text-xl font-bold mb-2 tracking-tight">{displayPlanText(plan.plan_name)}</h3>
                  <div className="px-3 py-1 glass bg-white/5 rounded-lg inline-block text-xs font-bold text-white/70 mb-6">
                    {plan.plan_month || selectedPlanMonth} / {plan.frequency}
                  </div>
                  {plan.rest_days && plan.rest_days.length > 0 && (
                    <div className="mb-6 rounded-2xl border border-secondary/20 bg-secondary/10 px-4 py-3 text-sm text-secondary">
                      休息日: {weekdayLabels(plan.rest_days)}
                    </div>
                  )}
                  
                  <div className="space-y-3">
                    {plan.weekly_routine.map((r: DayRoutine, i: number) => (
                      <div key={i} className="bg-black/30 p-4 rounded-xl border border-white/5 relative overflow-hidden group hover:border-primary/30 transition-colors">
                        <div className="absolute left-0 top-0 bottom-0 w-1 bg-primary/20 group-hover:bg-primary transition-colors" />
                        <div className="flex justify-between items-start mb-2">
                          <span className="text-[10px] font-black text-primary bg-primary/10 px-2 py-0.5 rounded-md">{displayPlanText(r.day_name)}</span>
                        </div>
                        <div className="font-bold text-sm mb-2">{displayPlanText(r.target)}</div>
                        <div className="flex flex-wrap gap-1">
                          {r.example_exercises.map((ex: string, eIdx: number) => (
                            <span key={eIdx} className="text-[10px] text-white/60 bg-white/5 px-2 py-1 rounded-md">
                              {ex}
                            </span>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ) : planView.status === "loading" ? (
                <p role="status" className="flex items-center gap-2 text-white/40 text-sm">
                  <Loader2 className="w-4 h-4 animate-spin" /> 月間プランを読み込み中...
                </p>
              ) : planView.status === "error" ? (
                <div role="alert" className="space-y-3 text-sm text-red-200">
                  <p>{selectedPlanMonth} の月間プランを読み込めませんでした。</p>
                  <button
                    type="button"
                    onClick={() => setPlanReloadKey(key => key + 1)}
                    className="px-3 py-1.5 rounded-xl bg-white/10 text-xs font-bold text-white hover:bg-white/20 transition-colors flex items-center gap-1.5"
                  >
                    <RefreshCw className="w-3.5 h-3.5" /> プランを再読み込み
                  </button>
                </div>
              ) : (
                <p className="text-white/40 text-sm">
                  {selectedPlanMonth} のプランはまだありません。<br/>
                  今月分はホームから生成できます。
                </p>
              )}
            </div>

            <button
              type="button"
              onClick={() => setEditingDate(formatLocalDate(today))}
              className="w-full text-left glass rounded-2xl p-6 bg-gradient-to-r from-green-500/10 to-transparent border-green-500/30 cursor-pointer flex justify-between items-center group hover:scale-[1.02] active:scale-[0.98] transition-transform"
            >
              <span className="block">
                <span className="block font-bold text-green-400 mb-1 group-hover:text-green-300 transition-colors">今日の記録</span>
                <span className="block text-xs text-white/50">今日を実施済みにする</span>
              </span>
              <span className="w-10 h-10 rounded-full bg-green-500/20 text-green-400 flex items-center justify-center group-hover:bg-green-500 group-hover:text-black transition-all">
                <Check className="w-5 h-5 font-bold" />
              </span>
            </button>
          </div>
        </div>
      </main>
      {editingDate && isFutureDateKey(editingDate) && (
        <CalendarPlanEditor
          date={editingDate}
          onClose={() => setEditingDate(null)}
          onSaved={() => {
            setCalendarReloadKey(key => key + 1);
            setEditingDate(null);
          }}
        />
      )}
      {editingDate && !isFutureDateKey(editingDate) && (
        <CalendarWorkoutEditor
          date={editingDate}
          onClose={() => setEditingDate(null)}
          onSaved={() => {
            setCalendarReloadKey(key => key + 1);
            setEditingDate(null);
          }}
        />
      )}
    </div>
  );
}

export function CalendarWorkoutEditor({
  date,
  onClose,
  onSaved,
}: {
  date: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  useBodyScrollLock();
  const [workout, setWorkout] = useState<CalendarWorkout | null>(null);
  const [title, setTitle] = useState("筋トレ");
  const [sets, setSets] = useState<CalendarWorkoutSet[]>([]);
  // 読み込んだ時点の内容。現在の入力と違えば、保存していない変更がある。
  const [loadedSnapshot, setLoadedSnapshot] = useState(() => workoutSnapshot("筋トレ", []));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [showExerciseSelector, setShowExerciseSelector] = useState(false);

  useEffect(() => {
    setLoading(true);
    setError("");
    apiFetch<CalendarWorkout>(`/api/workouts/by-date/${date}`)
      .then((data: CalendarWorkout) => {
        const loadedTitle = data.title || "筋トレ";
        const loadedSets = Array.isArray(data.sets) ? data.sets : [];
        setWorkout(data);
        setTitle(loadedTitle);
        setSets(loadedSets);
        setLoadedSnapshot(workoutSnapshot(loadedTitle, loadedSets));
      })
      .catch(err => {
        if (err instanceof ApiError && err.status === 404) {
          setWorkout({ workout_id: 0, date, title: "筋トレ", sets: [] });
          setTitle("筋トレ");
          setSets([]);
          setLoadedSnapshot(workoutSnapshot("筋トレ", []));
          return;
        }
        console.error(err);
        setError(apiErrorMessage(err, "記録の取得に失敗しました。"));
      })
      .finally(() => setLoading(false));
  }, [date]);

  const hasUnsavedChanges = !loading && workoutSnapshot(title, sets) !== loadedSnapshot;

  const requestClose = () => {
    if (hasUnsavedChanges && !window.confirm(UNSAVED_CHANGES_MESSAGE)) return;
    onClose();
  };

  const addExercise = (exercise: { id: string; name: string }) => {
    setSets(prev => [
      ...prev,
      {
        exercise_id: exercise.id,
        exercise_name: exercise.name,
        weight: 0,
        reps: 10,
        set_order: prev.length + 1,
        feeling: "",
      },
    ]);
    setShowExerciseSelector(false);
  };

  // 種目検索をやり直さずに、同じ種目・同じ重量と回数のセットを直後に足す。
  const addSameExerciseSet = (index: number) => {
    setSets(prev => {
      const source = prev[index];
      if (!source) return prev;
      const added: CalendarWorkoutSet = {
        exercise_id: source.exercise_id,
        exercise_name: source.exercise_name,
        weight: source.weight,
        reps: source.reps,
        set_order: index + 2,
        feeling: "",
      };
      return [...prev.slice(0, index + 1), added, ...prev.slice(index + 1)].map((set, i) => ({ ...set, set_order: i + 1 }));
    });
  };

  const updateSet = (index: number, patch: Partial<CalendarWorkoutSet>) => {
    setSets(prev => prev.map((set, i) => i === index ? { ...set, ...patch } : set));
  };

  const removeSet = (index: number) => {
    setSets(prev => prev.filter((_, i) => i !== index).map((set, i) => ({ ...set, set_order: i + 1 })));
  };

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      const payload = {
        title,
        sets: sets.map((set, i) => ({
          exercise_id: set.exercise_id,
          weight: Number(set.weight) || 0,
          reps: Number(set.reps) || 0,
          set_order: i + 1,
          feeling: set.feeling || "",
        })),
      };
      await apiFetch<CalendarWorkout>(`/api/workouts/by-date/${date}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      onSaved();
    } catch (err) {
      console.error(err);
      setError(apiErrorMessage(err, "保存時に通信エラーが発生しました。"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[90] bg-black/80 backdrop-blur-md p-4 flex items-center justify-center overflow-hidden overscroll-none" onClick={requestClose}>
      <div role="dialog" aria-label="記録を編集" className="w-full max-w-3xl max-h-[90vh] overflow-hidden rounded-[28px] border border-white/10 bg-[#111] shadow-2xl text-white" onClick={e => e.stopPropagation()}>
        <div className="p-5 border-b border-white/10 flex items-start justify-between gap-4 bg-[#171717]">
          <div>
            <div className="text-xs font-black text-primary mb-1">{date}</div>
            <h2 className="text-xl font-black tracking-tight">記録を編集</h2>
          </div>
          <button onClick={requestClose} className="p-2 rounded-xl text-white/50 hover:text-white hover:bg-white/10 transition-colors" aria-label="閉じる">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="p-5 md:p-6 overflow-y-auto max-h-[calc(90vh-88px)] space-y-5">
          {loading ? (
            <div className="py-16 flex justify-center text-primary">
              <Loader2 className="w-7 h-7 animate-spin" />
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-3">
                <div className="space-y-2">
                  <label className="text-[10px] font-black text-white/40">タイトル</label>
                  <input
                    value={title}
                    onChange={e => setTitle(e.target.value)}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-4 py-3 focus:outline-none focus:border-primary/50"
                    placeholder="例: 胸の日"
                  />
                </div>
                {workout?.workout_id && (
                  <Link
                    href={`/workouts/${workout.workout_id}`}
                    className="self-end px-4 py-3 rounded-2xl bg-white/5 border border-white/10 text-sm font-bold text-white/70 hover:text-white hover:bg-white/10 transition-colors flex items-center justify-center gap-2"
                  >
                    詳細 <ExternalLink className="w-4 h-4" />
                  </Link>
                )}
              </div>

              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="font-bold">セット</h3>
                  <p className="text-xs text-white/40">種目を検索して、重量・回数・感想を保存できます。</p>
                </div>
                <button
                  onClick={() => setShowExerciseSelector(true)}
                  className="px-4 py-2 rounded-xl bg-primary text-black text-sm font-black hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center gap-2"
                >
                  <Plus className="w-4 h-4" /> 種目
                </button>
              </div>

              {sets.length === 0 ? (
                <button
                  onClick={() => setShowExerciseSelector(true)}
                  className="w-full min-h-40 rounded-3xl border border-dashed border-white/15 bg-white/[0.03] text-white/50 hover:text-white hover:border-primary/40 transition-colors flex flex-col items-center justify-center gap-3"
                >
                  <Dumbbell className="w-8 h-8 text-primary" />
                  種目を追加して記録を作成
                </button>
              ) : (
                <div className="space-y-3">
                  {sets.map((set, index) => (
                    <div key={`${set.exercise_id}-${index}`} className="rounded-2xl border border-white/10 bg-black/30 p-4 space-y-3">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[10px] font-black text-primary">{index + 1}セット目</div>
                          <div className="font-bold">{set.exercise_name || set.exercise_id}</div>
                        </div>
                        <button
                          onClick={() => removeSet(index)}
                          className="p-2 rounded-xl text-white/40 hover:text-red-300 hover:bg-red-500/10 transition-colors"
                          aria-label="セットを削除"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>

                      <div className="grid grid-cols-2 md:grid-cols-[120px_120px_1fr] gap-3">
                        <label className="space-y-1">
                          <span className="text-[10px] font-black text-white/40">重量 kg</span>
                          <input
                            type="number"
                            min="0"
                            step="0.5"
                            value={set.weight}
                            onChange={e => updateSet(index, { weight: Number(e.target.value) })}
                            className="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2 focus:outline-none focus:border-primary/50"
                          />
                        </label>
                        <label className="space-y-1">
                          <span className="text-[10px] font-black text-white/40">回数</span>
                          <input
                            type="number"
                            min="1"
                            step="1"
                            value={set.reps}
                            onChange={e => updateSet(index, { reps: Number(e.target.value) })}
                            className="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2 focus:outline-none focus:border-primary/50"
                          />
                        </label>
                        <label className="space-y-1 col-span-2 md:col-span-1">
                          <span className="text-[10px] font-black text-white/40">感想</span>
                          <input
                            value={set.feeling}
                            onChange={e => updateSet(index, { feeling: e.target.value })}
                            className="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2 focus:outline-none focus:border-primary/50"
                            placeholder="例: かなり重い / 余裕あり"
                          />
                        </label>
                      </div>

                      {sets[index + 1]?.exercise_id !== set.exercise_id && (
                        <button
                          type="button"
                          onClick={() => addSameExerciseSet(index)}
                          className="px-3 py-2 rounded-xl bg-white/5 border border-white/10 text-xs font-bold text-white/70 hover:text-white hover:bg-white/10 transition-colors flex items-center gap-1.5"
                        >
                          <Plus className="w-3.5 h-3.5" /> 同じ種目でセットを追加
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              )}

              {error && (
                <div role="alert" className="rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200 whitespace-pre-line">
                  {error}
                </div>
              )}

              <div className="flex flex-col sm:flex-row gap-3 pt-2">
                <button
                  onClick={requestClose}
                  className="sm:flex-1 py-3 rounded-2xl bg-white/5 border border-white/10 font-bold text-white/70 hover:text-white hover:bg-white/10 transition-colors"
                >
                  キャンセル
                </button>
                <button
                  onClick={save}
                  disabled={saving || sets.length === 0}
                  className="sm:flex-1 py-3 rounded-2xl bg-primary text-black font-black hover:scale-[1.01] active:scale-[0.99] transition-all disabled:opacity-40 disabled:pointer-events-none flex items-center justify-center gap-2"
                >
                  {saving ? <Loader2 className="w-5 h-5 animate-spin" /> : <Save className="w-5 h-5" />}
                  保存
                </button>
              </div>
            </>
          )}
        </div>
      </div>
      {showExerciseSelector && (
        <div onClick={e => e.stopPropagation()}>
          <ExerciseSelectorModal
            onClose={() => setShowExerciseSelector(false)}
            onSelect={addExercise}
          />
        </div>
      )}
    </div>
  );
}

export function CalendarPlanEditor({
  date,
  onClose,
  onSaved,
}: {
  date: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  useBodyScrollLock();
  const [plan, setPlan] = useState<CalendarPlan | null>(null);
  // 月間プランも保存済みの予定もない日は、空の予定を新規作成する。
  const [isNew, setIsNew] = useState(false);
  const [title, setTitle] = useState("筋トレ");
  const [target, setTarget] = useState("筋トレ");
  const [duration, setDuration] = useState(45);
  const [coachNote, setCoachNote] = useState("");
  const [exercises, setExercises] = useState<CalendarPlanExercise[]>([]);
  // 読み込んだ時点の内容。現在の入力と違えば、保存していない変更がある。
  const [loadedSnapshot, setLoadedSnapshot] = useState(() => planSnapshot("筋トレ", "筋トレ", 45, "", []));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [showExerciseSelector, setShowExerciseSelector] = useState(false);

  useEffect(() => {
    const showPlan = (loaded: CalendarPlan | null) => {
      const content = loaded?.plan;
      const loadedTitle = content?.workout_title || "筋トレ";
      const loadedTarget = content?.target || content?.workout_title || "筋トレ";
      const loadedDuration = content?.estimated_duration_min || 45;
      const loadedCoachNote = content?.coach_note || "";
      const exerciseList = content?.exercises;
      const loadedExercises = Array.isArray(exerciseList) ? exerciseList : [];
      setPlan(loaded);
      setIsNew(loaded === null);
      setTitle(loadedTitle);
      setTarget(loadedTarget);
      setDuration(loadedDuration);
      setCoachNote(loadedCoachNote);
      setExercises(loadedExercises);
      setLoadedSnapshot(planSnapshot(loadedTitle, loadedTarget, loadedDuration, loadedCoachNote, loadedExercises));
    };

    setLoading(true);
    setError("");
    apiFetch<CalendarPlan>(`/api/workout-plans/${date}`)
      .then((data: CalendarPlan) => showPlan(data))
      .catch(err => {
        if (err instanceof ApiError && err.status === 404) {
          // 月間プランなしの利用では予定がまだないだけなので、エラーにせず新規作成へ進める。
          showPlan(null);
          return;
        }
        console.error(err);
        setError(apiErrorMessage(err, "予定の取得に失敗しました。"));
      })
      .finally(() => setLoading(false));
  }, [date]);

  const hasUnsavedChanges = !loading && planSnapshot(title, target, duration, coachNote, exercises) !== loadedSnapshot;

  const requestClose = () => {
    if (hasUnsavedChanges && !window.confirm(UNSAVED_CHANGES_MESSAGE)) return;
    onClose();
  };

  const addExercise = (exercise: { id: string; name: string }) => {
    setExercises(prev => [
      ...prev,
      {
        exercise_id: exercise.id,
        name: exercise.name,
        planned_sets: 3,
        target_weight: 0,
        target_reps: 10,
      },
    ]);
    setShowExerciseSelector(false);
  };

  const updateExercise = (index: number, patch: Partial<CalendarPlanExercise>) => {
    setExercises(prev => prev.map((exercise, i) => i === index ? { ...exercise, ...patch } : exercise));
  };

  const removeExercise = (index: number) => {
    setExercises(prev => prev.filter((_, i) => i !== index));
  };

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      await apiFetch<CalendarPlan>(`/api/workout-plans/${date}`, {
        method: "PUT",
        body: JSON.stringify({
          workout_title: title,
          target,
          estimated_duration_min: Number(duration) || 45,
          coach_note: coachNote,
          exercises: exercises.map(ex => ({
            exercise_id: ex.exercise_id,
            name: ex.name,
            planned_sets: Number(ex.planned_sets) || 3,
            target_weight: Number(ex.target_weight) || 0,
            target_reps: Number(ex.target_reps) || 10,
          })),
        }),
      });
      onSaved();
    } catch (err) {
      console.error(err);
      setError(apiErrorMessage(err, "保存時に通信エラーが発生しました。"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[90] bg-black/80 backdrop-blur-md p-4 flex items-center justify-center overflow-hidden overscroll-none" onClick={requestClose}>
      <div role="dialog" aria-label={isNew ? "予定を作成" : "予定を編集"} className="w-full max-w-3xl max-h-[90vh] overflow-hidden rounded-[28px] border border-white/10 bg-[#111] shadow-2xl text-white" onClick={e => e.stopPropagation()}>
        <div className="p-5 border-b border-white/10 flex items-start justify-between gap-4 bg-[#171717]">
          <div>
            <div className="text-xs font-black text-primary mb-1">{date}</div>
            <h2 className="text-xl font-black tracking-tight">{isNew ? "予定を作成" : "予定を編集"}</h2>
            {plan?.status === "draft" && <p className="text-xs text-white/40 mt-1">月間プランから作った未保存の予定です。</p>}
            {isNew && <p className="text-xs text-white/40 mt-1">この日の予定はまだありません。種目を追加して作成できます。</p>}
          </div>
          <button onClick={requestClose} className="p-2 rounded-xl text-white/50 hover:text-white hover:bg-white/10 transition-colors" aria-label="閉じる">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="p-5 md:p-6 overflow-y-auto max-h-[calc(90vh-88px)] space-y-5">
          {loading ? (
            <div className="py-16 flex justify-center text-primary">
              <Loader2 className="w-7 h-7 animate-spin" />
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-[1fr_1fr_120px] gap-3">
                <label className="space-y-2">
                  <span className="text-[10px] font-black text-white/40">タイトル</span>
                  <input
                    value={title}
                    onChange={e => setTitle(e.target.value)}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-4 py-3 focus:outline-none focus:border-primary/50"
                    placeholder="例: 胸の日"
                  />
                </label>
                <label className="space-y-2">
                  <span className="text-[10px] font-black text-white/40">狙い</span>
                  <input
                    value={target}
                    onChange={e => setTarget(e.target.value)}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-4 py-3 focus:outline-none focus:border-primary/50"
                    placeholder="例: 胸・肩"
                  />
                </label>
                <label className="space-y-2">
                  <span className="text-[10px] font-black text-white/40">目安分</span>
                  <input
                    type="number"
                    min="15"
                    step="5"
                    value={duration}
                    onChange={e => setDuration(Number(e.target.value))}
                    className="w-full bg-black/40 border border-white/10 rounded-2xl px-4 py-3 focus:outline-none focus:border-primary/50"
                  />
                </label>
              </div>

              <label className="space-y-2 block">
                <span className="text-[10px] font-black text-white/40">メモ</span>
                <textarea
                  value={coachNote}
                  onChange={e => setCoachNote(e.target.value)}
                  rows={2}
                  className="w-full bg-black/40 border border-white/10 rounded-2xl px-4 py-3 focus:outline-none focus:border-primary/50 resize-none"
                  placeholder="当日の自分へのメモ"
                />
              </label>

              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="font-bold">予定種目</h3>
                  <p className="text-xs text-white/40">未来の日付では、当日やる予定を調整します。</p>
                </div>
                <button
                  onClick={() => setShowExerciseSelector(true)}
                  className="px-4 py-2 rounded-xl bg-primary text-black text-sm font-black hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center gap-2"
                >
                  <Plus className="w-4 h-4" /> 種目
                </button>
              </div>

              {exercises.length === 0 ? (
                <button
                  onClick={() => setShowExerciseSelector(true)}
                  className="w-full min-h-40 rounded-3xl border border-dashed border-white/15 bg-white/[0.03] text-white/50 hover:text-white hover:border-primary/40 transition-colors flex flex-col items-center justify-center gap-3"
                >
                  <Dumbbell className="w-8 h-8 text-primary" />
                  種目を追加して予定を作成
                </button>
              ) : (
                <div className="space-y-3">
                  {exercises.map((exercise, index) => (
                    <div key={`${exercise.exercise_id}-${index}`} className="rounded-2xl border border-white/10 bg-black/30 p-4 space-y-3">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[10px] font-black text-primary">{index + 1}種目目</div>
                          <div className="font-bold">{exercise.name || exercise.exercise_id}</div>
                        </div>
                        <button
                          onClick={() => removeExercise(index)}
                          className="p-2 rounded-xl text-white/40 hover:text-red-300 hover:bg-red-500/10 transition-colors"
                          aria-label="予定種目を削除"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>

                      <div className="grid grid-cols-3 gap-3">
                        <label className="space-y-1">
                          <span className="text-[10px] font-black text-white/40">セット</span>
                          <input
                            type="number"
                            min="1"
                            step="1"
                            value={exercise.planned_sets}
                            onChange={e => updateExercise(index, { planned_sets: Number(e.target.value) })}
                            className="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2 focus:outline-none focus:border-primary/50"
                          />
                        </label>
                        <label className="space-y-1">
                          <span className="text-[10px] font-black text-white/40">重量 kg</span>
                          <input
                            type="number"
                            min="0"
                            step="0.5"
                            value={exercise.target_weight}
                            onChange={e => updateExercise(index, { target_weight: Number(e.target.value) })}
                            className="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2 focus:outline-none focus:border-primary/50"
                          />
                        </label>
                        <label className="space-y-1">
                          <span className="text-[10px] font-black text-white/40">回数</span>
                          <input
                            type="number"
                            min="1"
                            step="1"
                            value={exercise.target_reps}
                            onChange={e => updateExercise(index, { target_reps: Number(e.target.value) })}
                            className="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2 focus:outline-none focus:border-primary/50"
                          />
                        </label>
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {error && (
                <div role="alert" className="rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200 whitespace-pre-line">
                  {error}
                </div>
              )}

              <div className="flex flex-col sm:flex-row gap-3 pt-2">
                <button
                  onClick={requestClose}
                  className="sm:flex-1 py-3 rounded-2xl bg-white/5 border border-white/10 font-bold text-white/70 hover:text-white hover:bg-white/10 transition-colors"
                >
                  キャンセル
                </button>
                <button
                  onClick={save}
                  disabled={saving || exercises.length === 0}
                  className="sm:flex-1 py-3 rounded-2xl bg-primary text-black font-black hover:scale-[1.01] active:scale-[0.99] transition-all disabled:opacity-40 disabled:pointer-events-none flex items-center justify-center gap-2"
                >
                  {saving ? <Loader2 className="w-5 h-5 animate-spin" /> : <Save className="w-5 h-5" />}
                  予定を保存
                </button>
              </div>
            </>
          )}
        </div>
      </div>
      {showExerciseSelector && (
        <div onClick={e => e.stopPropagation()}>
          <ExerciseSelectorModal
            onClose={() => setShowExerciseSelector(false)}
            onSelect={addExercise}
          />
        </div>
      )}
    </div>
  );
}
