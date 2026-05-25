"use client";

/* eslint-disable @typescript-eslint/no-explicit-any */
import React from "react";
import Link from "next/link";
import { 
  Activity, 
  Flame, 
  Dumbbell, 
  Plus,
  Calendar,
  Settings,
  Sparkles,
  Loader2,
  X,
  Target,
  RefreshCw,
  ChevronRight,
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { AlternativeCoachModal } from "@/components/AlternativeCoachModal";
import { LogoutButton, useAuth } from "@/components/AuthGate";

function getCurrentPlanMonth() {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
}

const WEEKDAY_OPTIONS = [
  { value: 0, label: "日" },
  { value: 1, label: "月" },
  { value: 2, label: "火" },
  { value: 3, label: "水" },
  { value: 4, label: "木" },
  { value: 5, label: "金" },
  { value: 6, label: "土" },
];

const EQUIPMENT_OPTIONS = ["ダンベル", "バーベル", "マシン", "ケーブル", "自重", "チューブ・バンド", "ケトルベル", "メディシンボール", "EZバー", "その他"];
const ENVIRONMENT_OPTIONS = ["ジム", "家", "どちらも"];

function weekdayLabels(days?: number[]) {
  if (!days || days.length === 0) return "";
  return WEEKDAY_OPTIONS
    .filter(day => days.includes(day.value))
    .map(day => `${day.label}曜`)
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
    .replace(/^Day 5$/, "5日目");
}

function useBodyScrollLock() {
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
}

export default function Home() {
  const { user } = useAuth();
  const [showMonthlyModal, setShowMonthlyModal] = React.useState(false);
  const [showPreferencesModal, setShowPreferencesModal] = React.useState(false);
  const [monthlyPlan, setMonthlyPlan] = React.useState<any>(null);
  const [preferences, setPreferences] = React.useState<any>(null);
  const [dashboardData, setDashboardData] = React.useState<any>(null);
  const [altModalData, setAltModalData] = React.useState<{dayIdx: number, exIdx: number, exName: string, exId?: string} | null>(null);
  const [today, setToday] = React.useState<Date | null>(null);
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || '';
  const currentMonth = getCurrentPlanMonth();

  const saveMonthlyPlan = async (plan: any) => {
    const planToSave = { ...plan, user_id: user.id, plan_month: plan.plan_month || currentMonth };
    setMonthlyPlan(planToSave);
    try {
      const res = await fetch(`${apiUrl}/api/monthly-plan`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(planToSave)
      });
      if (res.ok) {
        const saved = await res.json();
        setMonthlyPlan(saved);
      }
    } catch (e) {
      console.error("Failed to save monthly plan:", e);
    }
  };

  React.useEffect(() => {
    setToday(new Date());
    fetch(`${apiUrl}/api/dashboard?user_id=${user.id}`)
      .then(res => res.json())
      .then(data => setDashboardData(data))
      .catch(console.error);

    fetch(`${apiUrl}/api/user-preferences?user_id=${user.id}`)
      .then(res => res.json())
      .then(data => setPreferences(data))
      .catch(console.error);

    fetch(`${apiUrl}/api/monthly-plan?user_id=${user.id}&month=${currentMonth}`)
      .then(async res => {
        if (res.status === 404) {
          setTimeout(() => setShowMonthlyModal(true), 1000);
          return null;
        }
        if (!res.ok) throw new Error(await res.text());
        return res.json();
      })
      .then(data => {
        if (data) setMonthlyPlan(data);
      })
      .catch(err => {
        console.error("Failed to fetch monthly plan:", err);
        setTimeout(() => setShowMonthlyModal(true), 1000);
      });
  }, [apiUrl, currentMonth, user.id]);

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 font-sans selection:bg-primary/30">
      <AnimatePresence>
        {showMonthlyModal && (
          <MonthlyPlanModal
            userId={user.id}
            onClose={() => setShowMonthlyModal(false)} 
            onPlanGenerated={(plan: any) => {
              setMonthlyPlan(plan);
              setShowMonthlyModal(false);
            }} 
          />
        )}
        {showPreferencesModal && (
          <TrainingPreferencesModal
            userId={user.id}
            initialPreferences={preferences}
            onClose={() => setShowPreferencesModal(false)}
            onSaved={(saved: any) => {
              setPreferences(saved);
              setShowPreferencesModal(false);
            }}
          />
        )}
        {altModalData && (
          <AlternativeCoachModal 
            exerciseId={altModalData.exId}
            exerciseName={altModalData.exName}
            onClose={() => setAltModalData(null)}
            onReplace={(newExercise: string | { id?: string; name?: string }) => {
              const newExName = typeof newExercise === "string" ? newExercise : newExercise.name || altModalData.exName;
              const updatedPlan = { ...monthlyPlan };
              updatedPlan.weekly_routine[altModalData.dayIdx].example_exercises[altModalData.exIdx] = newExName;
              if (typeof newExercise !== "string" && newExercise.id) {
                const routine = updatedPlan.weekly_routine[altModalData.dayIdx];
                routine.exercise_ids = Array.isArray(routine.exercise_ids) ? [...routine.exercise_ids] : [];
                routine.exercise_ids[altModalData.exIdx] = newExercise.id;
              }
              saveMonthlyPlan(updatedPlan);
              setAltModalData(null);
            }}
          />
        )}
      </AnimatePresence>

      {/* 背景の光 */}
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="max-w-7xl mx-auto relative z-10">
        {/* ヘッダー */}
        <header className="flex flex-col gap-5 sm:flex-row sm:justify-between sm:items-center mb-12">
          <div className="min-w-0">
            <h1 className="text-3xl sm:text-4xl font-bold tracking-tight leading-tight mb-1">
              おかえりなさい
              <span className="block text-gradient">{user.nickname}さん</span>
            </h1>
            <p className="text-white/50">今日のコンディションに合わせて進めましょう。</p>
          </div>
          <div className="flex flex-wrap gap-3 sm:gap-4 w-full sm:w-auto">
            <Link href="/calendar" className="flex-none p-3 glass rounded-2xl hover:bg-white/10 transition-colors block">
              <Calendar className="w-5 h-5 text-white/70" />
            </Link>
            <button
              onClick={() => setShowPreferencesModal(true)}
              className="flex-none p-3 glass rounded-2xl hover:bg-white/10 transition-colors"
              aria-label="トレーニング設定"
            >
              <Settings className="w-5 h-5 text-white/70" />
            </button>
            <LogoutButton />
            <Link href="/workout" className="w-full sm:w-auto px-6 py-3 bg-primary text-black font-bold rounded-2xl glow-primary hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center gap-2 justify-center">
              <Plus className="w-5 h-5" />
              <span>記録を始める</span>
            </Link>
          </div>
        </header>

        {/* 今日のワークアウト */}
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
                  <h2 className="text-2xl font-bold tracking-tight">今日のメニュー</h2>
                  <p className="text-white/50 text-sm font-medium">{today ? today.toLocaleDateString('ja-JP', { weekday: 'long', month: 'long', day: 'numeric' }) : ''}</p>
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

                const dayOfWeek = today ? today.getDay() : -1;
                const days = monthlyPlan.recommended_days || [1, 3, 5];
                const idx = days.indexOf(dayOfWeek);
                if (idx === -1) {
                  return (
                    <div className="bg-white/5 rounded-3xl p-8 text-center border border-white/10 relative overflow-hidden">
                      <div className="relative z-10">
                        <span className="text-4xl mb-4 block">☕️</span>
                        <h3 className="text-2xl font-black text-white mb-2">休息日</h3>
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
                        <span className="text-[10px] font-black tracking-widest text-primary bg-primary/20 px-3 py-1 rounded-full mb-3 inline-block">
                          {displayPlanText(routine.day_name)}
                        </span>
                        <h3 className="text-3xl md:text-4xl font-black tracking-tight text-white mb-2">{displayPlanText(routine.target)}</h3>
                        <p className="text-white/50 text-sm">
                          AIコーチが選んだ本日の最適メニュー
                          {monthlyPlan.rest_days?.length > 0 ? ` / 休息日: ${weekdayLabels(monthlyPlan.rest_days)}` : ""}
                        </p>
                      </div>
                      <Link href="/workout" className="w-full md:w-auto px-8 py-4 bg-primary text-black font-black rounded-2xl hover:scale-105 transition-transform flex justify-center items-center gap-2 shadow-[0_0_20px_rgba(255,170,0,0.3)]">
                        <Plus className="w-5 h-5" /> 筋トレを開始
                      </Link>
                    </div>
                    
                    <div className="space-y-3">
                      <p className="text-xs font-black text-white/30 tracking-widest mb-4">おすすめ種目</p>
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
                              onClick={() => setAltModalData({dayIdx: idx, exIdx: i, exName: ex, exId: routine.exercise_ids?.[i]})}
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

        {/* 最近の記録 */}
        <section className="glass rounded-[32px] p-6 md:p-10 border border-white/5">
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-xl font-bold flex items-center gap-2">
              <Activity className="w-5 h-5 text-secondary" /> 最近のワークアウト
            </h2>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {dashboardData?.recent_workouts?.length > 0 ? dashboardData.recent_workouts.map((w: any) => (
              <Link key={w.id} href={`/workouts/${w.id}`} className="bg-white/5 rounded-2xl p-5 border border-white/5 hover:bg-white/10 transition-colors group">
                <div className="flex justify-between items-start gap-3 mb-1">
                  <h4 className="font-bold truncate">{w.title}</h4>
                  <ChevronRight className="w-4 h-4 text-white/25 group-hover:text-primary transition-colors flex-shrink-0 mt-0.5" />
                </div>
                <p className="text-xs text-white/40 mb-4">{w.time}</p>
                <div>
                  <span className="text-[10px] font-bold text-white/30 block mb-1">時間</span>
                  <span className="font-medium text-sm">{w.duration}</span>
                </div>
              </Link>
            )) : dashboardData ? (
              <div className="col-span-full text-center py-8 text-white/40 text-sm bg-white/5 rounded-2xl border border-white/5">
                まだ完了したワークアウトはありません。
              </div>
            ) : (
              <div className="col-span-full text-center py-6 text-white/30 text-sm flex items-center justify-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" /> 記録を読み込み中...
              </div>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}


function TrainingPreferencesModal({ userId, initialPreferences, onClose, onSaved }: any) {
  useBodyScrollLock();

  const [preferredEquipment, setPreferredEquipment] = React.useState<string[]>(initialPreferences?.preferred_equipment || []);
  const [avoidedEquipment, setAvoidedEquipment] = React.useState<string[]>(initialPreferences?.avoided_equipment || []);
  const [trainingEnvironment, setTrainingEnvironment] = React.useState(initialPreferences?.training_environment || "ジム");
  const [notes, setNotes] = React.useState(initialPreferences?.notes || "");
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");

  const togglePreferred = (equipment: string) => {
    setPreferredEquipment(prev => (
      prev.includes(equipment)
        ? prev.filter(item => item !== equipment)
        : [...prev, equipment]
    ));
    setAvoidedEquipment(prev => prev.filter(item => item !== equipment));
  };

  const toggleAvoided = (equipment: string) => {
    setAvoidedEquipment(prev => (
      prev.includes(equipment)
        ? prev.filter(item => item !== equipment)
        : [...prev, equipment]
    ));
    setPreferredEquipment(prev => prev.filter(item => item !== equipment));
  };

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || '';
      const res = await fetch(`${apiUrl}/api/user-preferences`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_id: userId,
          preferred_equipment: preferredEquipment,
          avoided_equipment: avoidedEquipment,
          training_environment: trainingEnvironment,
          notes,
        })
      });
      if (!res.ok) {
        setError(await res.text() || "設定の保存に失敗しました。");
        return;
      }
      onSaved(await res.json());
    } catch (e) {
      console.error(e);
      setError("エラーが発生しました。");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-md overflow-hidden overscroll-none">
      <motion.div
        initial={{ opacity: 0, scale: 0.9, y: 20 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.9, y: 20 }}
        className="glass border-secondary/20 w-full max-w-2xl rounded-[32px] overflow-hidden relative shadow-2xl shadow-secondary/10 max-h-[90vh] overflow-y-auto overscroll-contain"
      >
        <button onClick={onClose} className="absolute top-6 right-6 p-2 bg-white/5 rounded-full hover:bg-white/10 transition z-10 text-white/60 hover:text-white">
          <X className="w-5 h-5" />
        </button>

        <div className="p-8 pb-28 sm:pb-8">
          <div className="w-16 h-16 bg-secondary/20 rounded-2xl flex items-center justify-center mb-6">
            <Settings className="w-8 h-8 text-secondary" />
          </div>
          <h2 className="text-2xl font-bold mb-2">トレーニング設定</h2>
          <p className="text-white/60 text-sm mb-6 leading-relaxed">
            AIが月間プランを作る時に参照します。よく使う器具や避けたい器具を入れておくと、現実にやりやすいメニューになります。
          </p>

          <div className="space-y-6 mb-8">
            <div className="space-y-3">
              <label className="text-xs font-black tracking-widest text-white/40 ml-1">トレーニング環境</label>
              <div className="grid grid-cols-3 gap-2">
                {ENVIRONMENT_OPTIONS.map(option => (
                  <button
                    key={option}
                    onClick={() => setTrainingEnvironment(option)}
                    className={`py-3 px-2 rounded-2xl border text-center font-bold text-sm transition-all ${
                      trainingEnvironment === option
                        ? 'bg-secondary/20 border-secondary text-secondary'
                        : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
                    }`}
                  >
                    {option}
                  </button>
                ))}
              </div>
            </div>

            <EquipmentPicker
              label="優先する器具"
              selected={preferredEquipment}
              onToggle={togglePreferred}
              activeClassName="bg-primary/20 border-primary text-primary"
            />

            <EquipmentPicker
              label="避けたい器具"
              selected={avoidedEquipment}
              onToggle={toggleAvoided}
              activeClassName="bg-red-500/20 border-red-500/40 text-red-200"
            />

            <div className="space-y-2">
              <label className="text-xs font-black tracking-widest text-white/40 ml-1">メモ</label>
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                className="w-full min-h-[110px] bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-secondary/50 text-sm"
                placeholder="例: スミスマシンはよく空いてる、懸垂バーなし、脚の日はマシン多めがいい"
              />
            </div>
          </div>

          {error && (
            <div className="mb-6 rounded-2xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200 whitespace-pre-line">
              {error}
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <button
              onClick={onClose}
              className="py-4 bg-white/10 text-white font-bold rounded-2xl hover:bg-white/15 transition-all"
            >
              キャンセル
            </button>
            <button
              onClick={save}
              disabled={saving}
              className="py-4 bg-secondary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50 disabled:pointer-events-none flex items-center justify-center gap-2"
            >
              {saving ? <Loader2 className="w-5 h-5 animate-spin" /> : <Settings className="w-5 h-5" />}
              保存する
            </button>
          </div>
        </div>
      </motion.div>
    </div>
  );
}

function EquipmentPicker({ label, selected, onToggle, activeClassName }: any) {
  return (
    <div className="space-y-3">
      <label className="text-xs font-black tracking-widest text-white/40 ml-1">{label}</label>
      <div className="flex flex-wrap gap-2">
        {EQUIPMENT_OPTIONS.map(equipment => (
          <button
            key={equipment}
            onClick={() => onToggle(equipment)}
            className={`px-4 py-2 rounded-xl border text-sm font-bold transition-all ${
              selected.includes(equipment)
                ? activeClassName
                : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
            }`}
          >
            {equipment}
          </button>
        ))}
      </div>
    </div>
  );
}


function MonthlyPlanModal({ userId, onClose, onPlanGenerated }: any) {
  useBodyScrollLock();

  const [motivation, setMotivation] = React.useState("健康維持");
  const [frequency, setFrequency] = React.useState("週3-4回");
  const [restDays, setRestDays] = React.useState<number[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const canGenerate = restDays.length < 7;

  const toggleRestDay = (day: number) => {
    setRestDays(prev => (
      prev.includes(day)
        ? prev.filter(value => value !== day)
        : [...prev, day].sort((a, b) => a - b)
    ));
  };

  const generatePlan = async () => {
    if (!canGenerate) return;
    setLoading(true);
    setError("");
    try {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || '';
      const res = await fetch(`${apiUrl}/api/monthly-plan`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_id: userId,
          plan_month: getCurrentPlanMonth(),
          motivation,
          frequency,
          rest_days: restDays
        })
      });
      if (!res.ok) {
        setError(await res.text() || "月間プランの作成に失敗しました。");
        return;
      }
      const data = await res.json();
      onPlanGenerated(data);
    } catch (e) {
      console.error(e);
      setError("エラーが発生しました。");
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
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-md overflow-hidden overscroll-none">
      <motion.div 
        initial={{ opacity: 0, scale: 0.9, y: 20 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.9, y: 20 }}
        className="glass border-primary/20 w-full max-w-lg rounded-[32px] overflow-hidden relative shadow-2xl shadow-primary/10 max-h-[90vh] overflow-y-auto overscroll-contain"
      >
        <button onClick={onClose} className="absolute top-6 right-6 p-2 bg-white/5 rounded-full hover:bg-white/10 transition z-10 text-white/60 hover:text-white">
          <X className="w-5 h-5" />
        </button>
        
        <div className="p-8 pb-28 sm:pb-8">
          <div className="w-16 h-16 bg-primary/20 rounded-2xl flex items-center justify-center mb-6">
            <Calendar className="w-8 h-8 text-primary" />
          </div>
          <h2 className="text-2xl font-bold mb-2">新しい月が始まりました！</h2>
          <p className="text-white/60 text-sm mb-6 leading-relaxed">
            今月の目標やペースを教えてください。AIコーチが最適なトレーニングルーティンを即座に設計します。
          </p>
          
          <div className="space-y-6 mb-8">
            <div className="space-y-3">
              <label className="text-xs font-black tracking-widest text-white/40 ml-1">今月のモチベーション</label>
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
              <label className="text-xs font-black tracking-widest text-white/40 ml-1">通える頻度（目安）</label>
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

            <div className="space-y-3">
              <label className="text-xs font-black tracking-widest text-white/40 ml-1">休みたい曜日（任意）</label>
              <div className="grid grid-cols-7 gap-2">
                {WEEKDAY_OPTIONS.map(day => (
                  <button
                    key={day.value}
                    onClick={() => toggleRestDay(day.value)}
                    className={`aspect-square rounded-2xl border text-center font-black text-sm transition-all ${
                      restDays.includes(day.value)
                        ? 'bg-secondary/20 border-secondary text-secondary'
                        : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
                    }`}
                  >
                    {day.label}
                  </button>
                ))}
              </div>
              <p className="text-[11px] text-white/40 leading-relaxed">
                選んだ曜日は休息日として避けます。生活リズム優先でOKです。
              </p>
              {!canGenerate && (
                <p className="text-[11px] text-red-300 leading-relaxed">
                  すべての曜日が休息日になっています。少なくとも1日は候補日を残してください。
                </p>
              )}
            </div>
          </div>

          {error && (
            <div className="mb-6 rounded-2xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200 whitespace-pre-line">
              {error}
            </div>
          )}

          <button 
            onClick={generatePlan}
            disabled={loading || !canGenerate}
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
