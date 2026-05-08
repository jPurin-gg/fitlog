"use client";

import React from "react";
import Link from "next/link";
import { 
  Activity, 
  Flame, 
  Dumbbell, 
  Timer, 
  TrendingUp, 
  ChevronRight, 
  Plus,
  Calendar,
  Settings,
  Bell,
  Sparkles,
  BrainCircuit,
  Loader2,
  X,
  Target,
  RefreshCw,
  MessageSquare,
  ArrowRight
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { AlternativeCoachModal } from "@/components/AlternativeCoachModal";

export default function Home() {
  const [showMonthlyModal, setShowMonthlyModal] = React.useState(false);
  const [monthlyPlan, setMonthlyPlan] = React.useState<any>(null);
  const [dashboardData, setDashboardData] = React.useState<any>(null);
  const [altModalData, setAltModalData] = React.useState<{dayIdx: number, exIdx: number, exName: string} | null>(null);

  React.useEffect(() => {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    fetch(`${apiUrl}/api/dashboard`)
      .then(res => res.json())
      .then(data => setDashboardData(data))
      .catch(console.error);

    // Check if we need to show the monthly plan modal
    const currentMonth = new Date().toISOString().slice(0, 7); // e.g. "2026-02"
    const savedMonth = localStorage.getItem('lastPlanMonth');
    const savedPlan = localStorage.getItem('currentMonthlyPlan');
    
    // Only show modal if no month is saved or if we are in a new month
    if (!savedMonth || savedMonth !== currentMonth) {
      setTimeout(() => setShowMonthlyModal(true), 1000); // slight delay for smooth UX
    } else if (savedPlan) {
      setMonthlyPlan(JSON.parse(savedPlan));
    }
  }, []);

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 font-sans selection:bg-primary/30">
      <AnimatePresence>
        {showMonthlyModal && (
          <MonthlyPlanModal 
            onClose={() => setShowMonthlyModal(false)} 
            onPlanGenerated={(plan: any) => {
              setMonthlyPlan(plan);
              const currentMonth = new Date().toISOString().slice(0, 7);
              localStorage.setItem('lastPlanMonth', currentMonth);
              localStorage.setItem('currentMonthlyPlan', JSON.stringify(plan));
              setShowMonthlyModal(false);
            }} 
          />
        )}
        {altModalData && (
          <AlternativeCoachModal 
            exerciseName={altModalData.exName}
            onClose={() => setAltModalData(null)}
            onReplace={(newExName: string) => {
              const updatedPlan = { ...monthlyPlan };
              updatedPlan.weekly_routine[altModalData.dayIdx].example_exercises[altModalData.exIdx] = newExName;
              setMonthlyPlan(updatedPlan);
              localStorage.setItem('currentMonthlyPlan', JSON.stringify(updatedPlan));
              setAltModalData(null);
            }}
          />
        )}
      </AnimatePresence>

      {/* Background Glows */}
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="max-w-7xl mx-auto relative z-10">
        {/* Header */}
        <header className="flex justify-between items-center mb-12">
          <div>
            <h1 className="text-4xl font-bold tracking-tight mb-1">
              Hey, <span className="text-gradient">Mitsuki</span> 👋
            </h1>
            <p className="text-white/50">Ready to crush your goals today?</p>
          </div>
          <div className="flex gap-4">
            <Link href="/calendar" className="p-3 glass rounded-2xl hover:bg-white/10 transition-colors block">
              <Calendar className="w-5 h-5 text-white/70" />
            </Link>
            <button className="p-3 glass rounded-2xl hover:bg-white/10 transition-colors">
              <Bell className="w-5 h-5 text-white/70" />
            </button>
            <button className="px-6 py-3 bg-primary text-black font-bold rounded-2xl glow-primary hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center gap-2">
              <Plus className="w-5 h-5" />
              <span>New Workout</span>
            </button>
          </div>
        </header>

        {/* Today's Workout Focus */}
        <section className="mb-8">
          <div className="glass rounded-[32px] p-6 md:p-10 relative overflow-hidden border border-white/10 shadow-2xl shadow-primary/5">
            <div className="absolute top-0 right-0 p-8 text-primary/5 pointer-events-none">
              <Dumbbell className="w-64 h-64 -rotate-12" />
            </div>

            <div className="relative z-10">
              <div className="flex items-center gap-3 mb-8">
                <div className="w-12 h-12 rounded-2xl bg-primary/20 flex items-center justify-center text-primary">
                  <Flame className="w-6 h-6" />
                </div>
                <div>
                  <h2 className="text-2xl font-bold tracking-tight">Today's Workout</h2>
                  <p className="text-white/50 text-sm font-medium">{new Date().toLocaleDateString('ja-JP', { weekday: 'long', month: 'long', day: 'numeric' })}</p>
                </div>
              </div>

              {(() => {
                if (!monthlyPlan) {
                  return (
                    <div className="text-center py-12">
                      <div className="inline-block p-4 rounded-full bg-white/5 mb-4">
                        <Target className="w-8 h-8 text-white/20" />
                      </div>
                      <h3 className="text-lg font-bold mb-2">プランを生成してください</h3>
                      <p className="text-white/50 text-sm">月初のアンケートに答えると、本日のメニューが表示されます。</p>
                    </div>
                  );
                }

                const dayOfWeek = new Date().getDay();
                const numDays = monthlyPlan.weekly_routine?.length || 3;
                let days = [];
                if (numDays === 1) days = [3];
                else if (numDays === 2) days = [2, 5];
                else if (numDays === 3) days = [1, 3, 5];
                else if (numDays === 4) days = [1, 2, 4, 5];
                else if (numDays === 5) days = [1, 2, 3, 5, 6];
                else days = [1, 2, 3, 4, 5, 6];

                // デバッグ用: 今日が必ず筋トレ日になるように強制設定しています（idxを0に固定）
                let idx = 0; // days.indexOf(dayOfWeek);
                if (idx === -1) {
                  return (
                    <div className="bg-white/5 rounded-3xl p-8 text-center border border-white/10 relative overflow-hidden">
                      <div className="relative z-10">
                        <span className="text-4xl mb-4 block">☕️</span>
                        <h3 className="text-2xl font-black text-white mb-2">Rest Day</h3>
                        <p className="text-white/60">今日はオフの日です。しっかり休んで回復しましょう。</p>
                      </div>
                    </div>
                  );
                }

                const routine = monthlyPlan.weekly_routine[idx];
                return (
                  <div className="bg-primary/10 rounded-3xl p-6 lg:p-8 border border-primary/20">
                    <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 mb-8">
                      <div>
                        <span className="text-[10px] font-black uppercase tracking-widest text-primary bg-primary/20 px-3 py-1 rounded-full mb-3 inline-block">
                          {routine.day_name}
                        </span>
                        <h3 className="text-3xl md:text-4xl font-black italic uppercase tracking-tight text-white mb-2">{routine.target}</h3>
                        <p className="text-white/50 text-sm">AIコーチが選んだ本日の最適メニュー</p>
                      </div>
                      <Link href="/workout" className="w-full md:w-auto px-8 py-4 bg-primary text-black font-black rounded-2xl hover:scale-105 transition-transform flex justify-center items-center gap-2 shadow-[0_0_20px_rgba(255,170,0,0.3)]">
                        <Plus className="w-5 h-5" /> 筋トレを開始
                      </Link>
                    </div>
                    
                    <div className="space-y-3">
                      <p className="text-xs font-black text-white/30 uppercase tracking-widest mb-4">Recommended Exercises</p>
                      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                        {routine.example_exercises.map((ex: string, i: number) => (
                          <div key={i} className="flex flex-col p-4 bg-black/40 rounded-2xl border border-white/5 transition-colors group">
                            <div className="flex items-center gap-4 mb-3">
                              <div className="w-12 h-12 bg-white/5 rounded-xl flex items-center justify-center text-white/20 group-hover:text-primary transition-all">
                                <Dumbbell className="w-6 h-6" />
                              </div>
                              <span className="font-bold text-lg text-white/90">{ex}</span>
                            </div>
                            <button 
                              onClick={() => setAltModalData({dayIdx: idx, exIdx: i, exName: ex})}
                              className="text-xs font-bold text-primary opacity-60 hover:opacity-100 flex items-center gap-1 mt-auto self-start bg-primary/10 px-3 py-1.5 rounded-lg border border-primary/20 transition-all"
                            >
                              <RefreshCw className="w-3 h-3" /> 種目を変更・AIに相談
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                );
              })()}
            </div>
          </div>
        </section>

        {/* Simplified Recent Workouts */}
        <section className="glass rounded-[32px] p-6 md:p-10 border border-white/5">
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-xl font-bold flex items-center gap-2">
              <Activity className="w-5 h-5 text-secondary" /> Recent Workouts
            </h2>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {dashboardData && dashboardData.recent_workouts ? dashboardData.recent_workouts.map((w: any) => (
              <div key={w.id} className="bg-white/5 rounded-2xl p-5 border border-white/5 hover:bg-white/10 transition-colors">
                <h4 className="font-bold mb-1 truncate">{w.title}</h4>
                <p className="text-xs text-white/40 mb-4">{w.time}</p>
                <div className="flex gap-6">
                  <div>
                    <span className="text-[10px] uppercase font-bold text-white/30 block mb-1">Duration</span>
                    <span className="font-medium text-sm">{w.duration}</span>
                  </div>
                  {/* Calorie mock logic was visible earlier but we keep it clean now */}
                  <div className="hidden">
                    <span className="text-[10px] uppercase font-bold text-white/30 block mb-1">Calories</span>
                    <span className="font-medium text-sm text-primary">{w.calories}</span>
                  </div>
                </div>
              </div>
            )) : (
              <div className="col-span-full text-center py-6 text-white/30 text-sm flex items-center justify-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" /> Loading records...
              </div>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}


function StatCard({ icon, label, value, unit, trend, color }: any) {
  return (
    <div className="glass rounded-[28px] p-6 hover:bg-white/5 transition-all group cursor-default">
      <div className="flex justify-between items-start mb-4">
        <div className="p-3 bg-white/5 rounded-2xl group-hover:scale-110 transition-transform duration-300">
          {icon}
        </div>
        <span className={`text-xs font-bold px-2 py-1 rounded-lg ${
          trend.startsWith('+') ? 'bg-green-500/10 text-green-400' : 'bg-red-500/10 text-red-400'
        }`}>
          {trend}
        </span>
      </div>
      <div>
        <p className="text-sm font-medium text-white/40 mb-1">{label}</p>
        <div className="flex items-baseline gap-1">
          <span className="text-2xl font-bold">{value}</span>
          <span className="text-xs font-medium text-white/30">{unit}</span>
        </div>
      </div>
    </div>
  );
}

function WorkoutItem({ title, type, duration, calories, time }: any) {
  return (
    <div className="glass border-white/5 rounded-2xl p-4 flex items-center justify-between group hover:bg-white/5 transition-all cursor-pointer">
      <div className="flex items-center gap-4">
        <div className="w-12 h-12 rounded-xl bg-white/5 flex items-center justify-center group-hover:bg-primary/10 transition-colors">
          <Activity className="w-6 h-6 text-white/40 group-hover:text-primary transition-colors" />
        </div>
        <div>
          <h4 className="font-bold text-sm">{title}</h4>
          <p className="text-[10px] text-white/40 font-medium uppercase tracking-wider">{type} • {time}</p>
        </div>
      </div>
      <div className="flex items-center gap-8">
        <div className="text-right">
          <p className="text-sm font-bold">{duration}</p>
          <p className="text-[10px] text-white/40">{calories}</p>
        </div>
        <ChevronRight className="w-5 h-5 text-white/20 group-hover:text-white transition-colors" />
      </div>
    </div>
  );
}

function MonthlyPlanModal({ onClose, onPlanGenerated }: any) {
  const [motivation, setMotivation] = React.useState("健康維持");
  const [frequency, setFrequency] = React.useState("週3-4回");
  const [loading, setLoading] = React.useState(false);

  const generatePlan = async () => {
    setLoading(true);
    try {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
      const res = await fetch(`${apiUrl}/api/monthly-plan`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ motivation, frequency })
      });
      const data = await res.json();
      onPlanGenerated(data);
    } catch (e) {
      console.error(e);
      // Fallback
      onPlanGenerated({
        plan_name: "PPL法 (Push/Pull/Legs)",
        frequency: "週3〜4回",
        description: "押す筋肉、引く筋肉、脚の3グループに分けて鍛える、最もバランスが良く結果が出やすい王道のルーティンです。",
        rationale: "バランス良く鍛えられるPPL法が最も適していると判断しました。まずはこれをベースに頑張りましょう！",
        weekly_routine: [
          { day_name: "Day 1", target: "Push", example_exercises: ["ベンチプレス", "ショルダープレス"] },
          { day_name: "Day 2", target: "Pull", example_exercises: ["懸垂", "ラットプルダウン"] },
          { day_name: "Day 3", target: "Legs", example_exercises: ["スクワット", "レッグプレス"] }
        ]
      });
    } finally {
      setLoading(false);
    }
  };

  const motivationOptions = [
    { id: "健康維持", label: "健康維持 / 気軽に", desc: "無理なく続けたい" },
    { id: "筋肉を大きく", label: "筋肉を大きく", desc: "ボディメイク重視" },
    { id: "本気", label: "本気で追い込む", desc: "限界突破したい" }
  ];

  const frequencyOptions = [
    { id: "週1-2回", label: "週1〜2回" },
    { id: "週3-4回", label: "週3〜4回" },
    { id: "週5-6回", label: "週5〜6回" }
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-md">
      <motion.div 
        initial={{ opacity: 0, scale: 0.9, y: 20 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.9, y: 20 }}
        className="glass border-primary/20 w-full max-w-lg rounded-[32px] overflow-hidden relative shadow-2xl shadow-primary/10"
      >
        <button onClick={onClose} className="absolute top-6 right-6 p-2 bg-white/5 rounded-full hover:bg-white/10 transition z-10 text-white/60 hover:text-white">
          <X className="w-5 h-5" />
        </button>
        
        <div className="p-8">
          <div className="w-16 h-16 bg-primary/20 rounded-2xl flex items-center justify-center mb-6">
            <Calendar className="w-8 h-8 text-primary" />
          </div>
          <h2 className="text-2xl font-bold mb-2">新しい月が始まりました！</h2>
          <p className="text-white/60 text-sm mb-6 leading-relaxed">
            今月の目標やペースを教えてください。AIコーチが最適なトレーニングルーティンを即座に設計します。
          </p>
          
          <div className="space-y-6 mb-8">
            <div className="space-y-3">
              <label className="text-xs font-black uppercase tracking-widest text-white/40 ml-1">今月のモチベーション</label>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
                {motivationOptions.map(opt => (
                  <button
                    key={opt.id}
                    onClick={() => setMotivation(opt.id)}
                    className={`p-3 rounded-2xl border text-left transition-all ${
                      motivation === opt.id 
                        ? 'bg-primary/20 border-primary text-white shadow-[0_0_15px_rgba(255,255,255,0.1)]' 
                        : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
                    }`}
                  >
                    <div className={`font-bold text-sm mb-1 ${motivation === opt.id ? 'text-primary' : ''}`}>{opt.label}</div>
                    <div className="text-[10px] opacity-70">{opt.desc}</div>
                  </button>
                ))}
              </div>
            </div>

            <div className="space-y-3">
              <label className="text-xs font-black uppercase tracking-widest text-white/40 ml-1">通える頻度 (目安)</label>
              <div className="grid grid-cols-3 gap-2">
                {frequencyOptions.map(opt => (
                  <button
                    key={opt.id}
                    onClick={() => setFrequency(opt.id)}
                    className={`py-3 px-2 rounded-2xl border text-center font-bold text-sm transition-all ${
                      frequency === opt.id 
                        ? 'bg-primary/20 border-primary text-white' 
                        : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
                    }`}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <button 
            onClick={generatePlan}
            disabled={loading}
            className="group relative w-full py-5 mt-2 bg-primary text-black font-black rounded-2xl shadow-[0_0_30px_rgba(255,170,0,0.5)] hover:shadow-[0_0_50px_rgba(255,170,0,0.7)] hover:scale-[1.02] active:scale-[0.98] transition-all duration-300 disabled:opacity-50 disabled:pointer-events-none tracking-widest overflow-hidden"
          >
            {/* Shimmer effect inside button */}
            <div className="absolute inset-0 -translate-x-full group-hover:animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-transparent via-white/30 to-transparent skew-x-12" />
            
            <div className="relative z-10 flex flex-col items-center justify-center gap-1">
              {loading ? (
                <Loader2 className="w-7 h-7 animate-spin" />
              ) : (
                <div className="flex items-center gap-2">
                  <span className="text-xl">決定してプランを作成</span>
                  <Sparkles className="w-6 h-6" />
                </div>
              )}
            </div>
          </button>
        </div>
      </motion.div>
    </div>
  );
}
