"use client";

import React, { useEffect, useState } from "react";
import { ArrowLeft, Calendar as CalendarIcon, Dumbbell, Sparkles, Check } from "lucide-react";
import Link from "next/link";
import { motion } from "framer-motion";

interface DayRoutine {
  day_name: string;
  target: string;
  example_exercises: string[];
}

interface MonthlyPlan {
  plan_name: string;
  frequency: string;
  description: string;
  rationale: string;
  recommended_days: number[];
  weekly_routine: DayRoutine[];
}

interface WorkedOutDay {
  date: number;
  type: string;
}


export default function CalendarPage() {
  const [currentDate, setCurrentDate] = useState(new Date());
  const [plan, setPlan] = useState<MonthlyPlan | null>(null);
  const [plannedWeekDays, setPlannedWeekDays] = useState<number[]>([]);
  const [workedOutDates, setWorkedOutDates] = useState<number[]>([]);
  const [workedOutDays, setWorkedOutDays] = useState<WorkedOutDay[]>([]);

  useEffect(() => {
    // Load plan
    const savedPlan = localStorage.getItem('currentMonthlyPlan');
    let generatedPlanDays: number[] = [];
    if (savedPlan) {
      const p = JSON.parse(savedPlan);
      setPlan(p);
      
      const days = p.recommended_days || [1, 3, 5];
      setPlannedWeekDays(days);
      generatedPlanDays = days;
    }
    // Fetch worked out dates from the DB
    const today = new Date();
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    fetch(`${apiUrl}/api/calendar?year=${today.getFullYear()}&month=${today.getMonth() + 1}`)
      .then(res => res.json())
      .then(data => {
        if (data && data.worked_out_dates) {
          setWorkedOutDates(data.worked_out_dates);
        }
        if (data && data.worked_out_days) {
          setWorkedOutDays(data.worked_out_days);
        }
      })
      .catch(err => console.error("Failed to fetch calendar data:", err));
  }, []);

  const year = currentDate.getFullYear();
  const month = currentDate.getMonth();

  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const firstDayOfMonth = new Date(year, month, 1).getDay(); // 0 is Sun, 1 is Mon...

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
    if (!dateNum) return { isToday: false, isWorkedOut: false, isPlanned: false, type: "" };
    const dateObj = new Date(year, month, dateNum);
    const isToday = isCurrentMonth && dateNum === today.getDate();
    const isWorkedOut = workedOutDates.includes(dateNum);
    
    // Day of week 0-6
    const dow = dateObj.getDay();
    const plannedIdx = plannedWeekDays.indexOf(dow);
    const isPlanned = plannedIdx !== -1;

    let plannedTarget = "";
    if (isPlanned && plan?.weekly_routine?.[plannedIdx]) {
      plannedTarget = plan.weekly_routine[plannedIdx].target;
      // Truncate some common long strings for better calendar fit
      if (plannedTarget.includes("Push")) plannedTarget = "Push";
      if (plannedTarget.includes("Pull")) plannedTarget = "Pull";
      if (plannedTarget.includes("Legs")) plannedTarget = "Legs";
    }

    const workedOutDay = workedOutDays.find(d => d.date === dateNum);
    const type = workedOutDay ? workedOutDay.type : "";

    return { 
      isToday, 
      isWorkedOut, 
      type,
      plannedTarget,
      isPlanned: isPlanned && !isWorkedOut && dateObj >= new Date(today.getFullYear(), today.getMonth(), today.getDate()) 
    };
  };

  const weekDayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 font-sans selection:bg-primary/30">
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="max-w-5xl mx-auto relative z-10">
        <header className="flex items-center justify-between mb-10">
          <div className="flex items-center gap-4">
            <Link href="/" className="p-3 glass rounded-2xl hover:bg-white/10 transition-colors">
              <ArrowLeft className="w-5 h-5" />
            </Link>
            <div>
              <h1 className="text-3xl font-bold tracking-tight">Schedule</h1>
              <p className="text-white/50">Your monthly workout plan</p>
            </div>
          </div>
        </header>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2">
            <div className="glass rounded-[32px] p-6 lg:p-8">
              <div className="flex justify-between items-center mb-8">
                <h2 className="text-2xl font-black uppercase tracking-widest text-primary flex items-center gap-3">
                  <CalendarIcon className="w-6 h-6" />
                  {currentDate.toLocaleString('default', { month: 'long', year: 'numeric' })}
                </h2>
                
                <div className="flex gap-4 text-[10px] md:text-xs font-bold text-white/60">
                  <div className="flex items-center gap-1 md:gap-2">
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full bg-green-500 shadow-[0_0_10px_rgba(34,197,94,0.5)]"></div>
                    Done
                  </div>
                  <div className="flex items-center gap-1 md:gap-2">
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full bg-primary shadow-[0_0_10px_rgba(255,170,0,0.5)]"></div>
                    Planned
                  </div>
                  <div className="flex items-center gap-1 md:gap-2">
                    <div className="w-2 h-2 md:w-3 md:h-3 rounded-full border-2 border-white"></div>
                    Today
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-7 gap-2 lg:gap-4 mb-4">
                {weekDayNames.map(day => (
                  <div key={day} className="text-center text-[10px] font-black uppercase text-white/30 tracking-widest">
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

                  return (
                    <motion.div 
                      key={day}
                      initial={{ opacity: 0, scale: 0.9 }}
                      animate={{ opacity: 1, scale: 1 }}
                      transition={{ delay: idx * 0.01 }}
                      className={`relative aspect-square rounded-2xl border flex flex-col items-center justify-center transition-all hover:scale-105 cursor-pointer group ${bgClass}`}
                    >
                      {status.isPlanned && !status.isWorkedOut && (
                        <div className="absolute inset-0 bg-gradient-to-tr from-primary/10 to-transparent opacity-50" />
                      )}
                      
                      <span className={`text-sm md:text-xl lg:text-2xl mb-1 ${textClass}`}>{day}</span>
                      
                      <div className="h-4 flex items-center justify-center">
                        {status.isWorkedOut ? (
                          <div className="flex flex-col items-center">
                            <span className="text-[8px] md:text-[10px] font-black text-green-300 uppercase leading-none text-center whitespace-normal px-1 w-full line-clamp-2">
                              {status.type}
                            </span>
                          </div>
                        ) : (
                          <>
                            {status.isPlanned && !status.isToday && (
                              <div className="flex flex-col items-center">
                                <span className="text-[7px] md:text-[9px] font-bold text-primary opacity-80 uppercase leading-tight text-center whitespace-normal px-0.5 w-full line-clamp-2">
                                  {status.plannedTarget || <Dumbbell className="w-3 h-3 mx-auto" />}
                                </span>
                              </div>
                            )}
                            {status.isToday && status.isPlanned && (
                              <div className="flex flex-col items-center animate-pulse">
                                <span className="text-[8px] md:text-[10px] font-black text-white uppercase leading-tight text-center whitespace-normal px-0.5 w-full line-clamp-2">
                                  {status.plannedTarget || <Dumbbell className="w-4 h-4 mx-auto" />}
                                </span>
                              </div>
                            )}
                          </>
                        )}
                      </div>
                    </motion.div>
                  );
                })}
              </div>
            </div>
          </div>

          <div className="space-y-6">
            <div className="glass rounded-[32px] p-6 lg:p-8 border-l-4 border-primary">
              <div className="flex items-center gap-2 text-primary font-bold mb-4 uppercase text-sm tracking-widest">
                <Sparkles className="w-5 h-5" /> Active Routine
              </div>
              {plan ? (
                <div>
                  <h3 className="text-xl font-bold mb-2 tracking-tight">{plan.plan_name}</h3>
                  <div className="px-3 py-1 glass bg-white/5 rounded-lg inline-block text-xs font-bold text-white/70 mb-6">
                    {plan.frequency}
                  </div>
                  
                  <div className="space-y-3">
                    {plan.weekly_routine.map((r: DayRoutine, i: number) => (
                      <div key={i} className="bg-black/30 p-4 rounded-xl border border-white/5 relative overflow-hidden group hover:border-primary/30 transition-colors">
                        <div className="absolute left-0 top-0 bottom-0 w-1 bg-primary/20 group-hover:bg-primary transition-colors" />
                        <div className="flex justify-between items-start mb-2">
                          <span className="text-[10px] uppercase font-black text-primary bg-primary/10 px-2 py-0.5 rounded-md">{r.day_name}</span>
                        </div>
                        <div className="font-bold text-sm mb-2">{r.target}</div>
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
                <p className="text-white/40 text-sm">プランがまだ生成されていません。<br/>ホームに戻って生成してください。</p>
              )}
            </div>
            
            <motion.div 
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              className="glass rounded-2xl p-6 bg-gradient-to-r from-green-500/10 to-transparent border-green-500/30 cursor-pointer flex justify-between items-center group"
            >
              <div>
                <h4 className="font-bold text-green-400 mb-1 group-hover:text-green-300 transition-colors">Log Workout</h4>
                <p className="text-xs text-white/50">Mark today as done</p>
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
