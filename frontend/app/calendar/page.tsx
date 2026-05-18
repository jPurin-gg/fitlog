"use client";

import React, { useEffect, useState } from "react";
import { ArrowLeft, Calendar as CalendarIcon, ChevronLeft, ChevronRight, Dumbbell, Sparkles, Check } from "lucide-react";
import Link from "next/link";
import { motion } from "framer-motion";
import { useAuth } from "@/components/AuthGate";

interface DayRoutine {
  day_name: string;
  target: string;
  example_exercises: string[];
  exercise_ids?: string[];
}

interface MonthlyPlan {
  id?: number;
  user_id?: number;
  plan_month?: string;
  plan_name: string;
  frequency: string;
  description: string;
  rationale: string;
  rest_days?: number[];
  recommended_days: number[];
  weekly_routine: DayRoutine[];
}

interface WorkedOutDay {
  date: number;
  workout_id: number;
  type: string;
}

function getCurrentPlanMonth() {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
}

function formatPlanMonth(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
}

function dateFromPlanMonth(planMonth: string) {
  const [year, month] = planMonth.split("-").map(Number);
  return new Date(year, (month || 1) - 1, 1);
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

function displayPlanText(text?: string) {
  if (!text) return "";
  return text
    .replace("PPL法 (Push/Pull/Legs)", "PPL法（押す・引く・脚）")
    .replace("Full Body", "全身")
    .replace("Push (胸・肩・三頭)", "押す日（胸・肩・三頭）")
    .replace("Pull (背中・二頭)", "引く日（背中・二頭）")
    .replace("Legs (脚・腹)", "脚の日（脚・腹）")
    .replace("全身 A", "全身その1")
    .replace("全身 B", "全身その2")
    .replace(/^Day 1$/, "1日目")
    .replace(/^Day 2$/, "2日目")
    .replace(/^Day 3$/, "3日目")
    .replace(/^Day 4$/, "4日目")
    .replace(/^Day 5$/, "5日目")
    .replace(/^Push$/, "押す日")
    .replace(/^Pull$/, "引く日")
    .replace(/^Legs$/, "脚の日")
    .replace(/^Strength$/, "筋トレ")
    .replace(/^Workout$/, "ワークアウト");
}

export default function CalendarPage() {
  const { user } = useAuth();
  const [currentDate, setCurrentDate] = useState(new Date());
  const [plan, setPlan] = useState<MonthlyPlan | null>(null);
  const [planHistory, setPlanHistory] = useState<MonthlyPlan[]>([]);
  const [plannedWeekDays, setPlannedWeekDays] = useState<number[]>([]);
  const [workedOutDates, setWorkedOutDates] = useState<number[]>([]);
  const [workedOutDays, setWorkedOutDays] = useState<WorkedOutDay[]>([]);
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || '';
  const selectedPlanMonth = formatPlanMonth(currentDate);

  useEffect(() => {
    fetch(`${apiUrl}/api/monthly-plans?user_id=${user.id}`)
      .then(res => {
        if (!res.ok) throw new Error("Failed to fetch monthly plan history");
        return res.json();
      })
      .then(data => setPlanHistory(Array.isArray(data) ? data : []))
      .catch(err => console.error("Failed to fetch monthly plan history:", err));
  }, [apiUrl, user.id]);

  useEffect(() => {
    fetch(`${apiUrl}/api/monthly-plan?user_id=${user.id}&month=${selectedPlanMonth}`)
      .then(res => {
        if (res.status === 404) return null;
        if (!res.ok) throw new Error("Failed to fetch monthly plan");
        return res.json();
      })
      .then(data => {
        if (!data) {
          setPlan(null);
          setPlannedWeekDays([]);
          return;
        }
        setPlan(data);
        setPlannedWeekDays(data.recommended_days || [1, 3, 5]);
      })
      .catch(err => console.error("Failed to fetch monthly plan:", err));

    // DBから実施済みの日付を取得する
    fetch(`${apiUrl}/api/calendar?user_id=${user.id}&year=${currentDate.getFullYear()}&month=${currentDate.getMonth() + 1}`)
      .then(res => res.json())
      .then(data => {
        if (data && data.worked_out_dates) {
          setWorkedOutDates(data.worked_out_dates);
        } else {
          setWorkedOutDates([]);
        }
        if (data && data.worked_out_days) {
          setWorkedOutDays(data.worked_out_days);
        } else {
          setWorkedOutDays([]);
        }
      })
      .catch(err => console.error("Failed to fetch calendar data:", err));
  }, [apiUrl, currentDate, selectedPlanMonth, user.id]);

  const year = currentDate.getFullYear();
  const month = currentDate.getMonth();
  const isSelectedCurrentMonth = selectedPlanMonth === getCurrentPlanMonth();

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
  const isCurrentMonth = today.getFullYear() === year && today.getMonth() === month;

  const getDayStatus = (dateNum: number | null) => {
    if (!dateNum) return { isToday: false, isWorkedOut: false, isPlanned: false, type: "", workoutId: null as number | null };
    const dateObj = new Date(year, month, dateNum);
    const isToday = isCurrentMonth && dateNum === today.getDate();
    const isWorkedOut = workedOutDates.includes(dateNum);
    
    const dow = dateObj.getDay();
    const plannedIdx = plannedWeekDays.indexOf(dow);
    const isPlanned = plannedIdx !== -1;

    let plannedTarget = "";
    if (isPlanned && plan?.weekly_routine?.[plannedIdx]) {
      plannedTarget = plan.weekly_routine[plannedIdx].target;
      // カレンダー内に収まるよう、代表的な分割名は短く表示する。
      if (plannedTarget.includes("Push")) plannedTarget = "押す日";
      if (plannedTarget.includes("Pull")) plannedTarget = "引く日";
      if (plannedTarget.includes("Legs")) plannedTarget = "脚の日";
    }

    const workedOutDay = workedOutDays.find(d => d.date === dateNum);
    const type = workedOutDay ? displayPlanText(workedOutDay.type) : "";
    const workoutId = workedOutDay?.workout_id || null;

    return { 
      isToday, 
      isWorkedOut, 
      type,
      workoutId,
      plannedTarget,
      isPlanned: isPlanned && !isWorkedOut && (!isCurrentMonth || dateObj >= new Date(today.getFullYear(), today.getMonth(), today.getDate())) 
    };
  };

  const weekDayNames = ["日", "月", "火", "水", "木", "金", "土"];
  const moveMonth = (offset: number) => {
    setCurrentDate(prev => new Date(prev.getFullYear(), prev.getMonth() + offset, 1));
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
              <div className="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between mb-8">
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
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full border-2 border-white"></div>
                    今日
                  </div>
                </div>
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
                  
                  if (status.isWorkedOut) {
                    bgClass = "bg-green-500/20 border-green-500/30 ring-1 ring-green-500/50";
                    textClass = "text-green-400 font-bold";
                  } else if (status.isPlanned) {
                    bgClass = "bg-primary/20 border-primary/30 ring-1 ring-primary/50 relative overflow-hidden";
                    textClass = "text-primary font-bold";
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
                        status.workoutId ? "hover:scale-105 cursor-pointer group" : "cursor-default"
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
                          </>
                        )}
                      </div>
                    </motion.div>
                  );

                  if (status.workoutId) {
                    return (
                      <Link
                        key={day}
                        href={`/workouts/${status.workoutId}`}
                        aria-label={`${selectedPlanMonth}-${String(day).padStart(2, "0")} のワークアウト履歴を見る`}
                        className="block"
                      >
                        {dayContent}
                      </Link>
                    );
                  }

                  return dayContent;
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
                          {historyPlan.plan_month === getCurrentPlanMonth() && (
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
              ) : (
                <p className="text-white/40 text-sm">
                  {selectedPlanMonth} のプランはまだありません。<br/>
                  今月分はホームから生成できます。
                </p>
              )}
            </div>
            
            <motion.div 
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              className="glass rounded-2xl p-6 bg-gradient-to-r from-green-500/10 to-transparent border-green-500/30 cursor-pointer flex justify-between items-center group"
            >
              <div>
                <h4 className="font-bold text-green-400 mb-1 group-hover:text-green-300 transition-colors">今日の記録</h4>
                <p className="text-xs text-white/50">今日を実施済みにする</p>
              </div>
              <div className="w-10 h-10 rounded-full bg-green-500/20 text-green-400 flex items-center justify-center group-hover:bg-green-500 group-hover:text-black transition-all">
                <Check className="w-5 h-5 font-bold" />
              </div>
            </motion.div>
          </div>
        </div>
      </main>
    </div>
  );
}
