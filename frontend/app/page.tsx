"use client";

import React from "react";
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
  Loader2
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";

export default function Home() {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 md:p-8 font-sans selection:bg-primary/30">
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
            <button className="p-3 glass rounded-2xl hover:bg-white/10 transition-colors">
              <Bell className="w-5 h-5 text-white/70" />
            </button>
            <button className="px-6 py-3 bg-primary text-black font-bold rounded-2xl glow-primary hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center gap-2">
              <Plus className="w-5 h-5" />
              <span>New Workout</span>
            </button>
          </div>
        </header>

        {/* Stats Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-12">
          <StatCard 
            icon={<Flame className="w-6 h-6 text-accent" />} 
            label="Calories" 
            value="2,450" 
            unit="kcal" 
            trend="+12%" 
            color="accent" 
          />
          <StatCard 
            icon={<Activity className="w-6 h-6 text-primary" />} 
            label="Heart Rate" 
            value="72" 
            unit="bpm" 
            trend="-2%" 
            color="primary" 
          />
          <StatCard 
            icon={<Timer className="w-6 h-6 text-secondary" />} 
            label="Active Time" 
            value="45" 
            unit="min" 
            trend="+5min" 
            color="secondary" 
          />
          <StatCard 
            icon={<TrendingUp className="w-6 h-6 text-green-400" />} 
            label="Weight" 
            value="68.5" 
            unit="kg" 
            trend="-0.5kg" 
            color="green-400" 
          />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Main Chart Area */}
          <div className="lg:col-span-2 space-y-8">
            <section className="glass rounded-[32px] p-8 relative overflow-hidden">
              <div className="flex justify-between items-center mb-8">
                <h2 className="text-xl font-bold">Activity Overview</h2>
                <div className="flex gap-2">
                  <span className="px-3 py-1 bg-white/5 rounded-lg text-xs font-medium">Week</span>
                  <span className="px-3 py-1 bg-white/10 rounded-lg text-xs font-medium">Month</span>
                </div>
              </div>
              
              {/* Mock Chart */}
              <div className="h-[300px] w-full flex items-end justify-between gap-2 px-2">
                {[40, 70, 45, 90, 65, 80, 55].map((h, i) => (
                  <div key={i} className="flex-1 flex flex-col items-center gap-4 group">
                    <div 
                      className="w-full bg-white/5 rounded-t-xl relative overflow-hidden transition-all duration-500 group-hover:bg-white/10"
                      style={{ height: `${h}%` }}
                    >
                      <div 
                        className="absolute bottom-0 left-0 w-full bg-gradient-to-t from-primary/40 to-primary/80 transition-all duration-700 delay-[i*100ms] group-hover:from-primary/60 group-hover:to-primary"
                        style={{ height: '0%' }}
                        ref={(el) => {
                          if (el) setTimeout(() => el.style.height = '100%', 100);
                        }}
                      />
                    </div>
                    <span className="text-[10px] text-white/40 font-medium">
                      {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'][i]}
                    </span>
                  </div>
                ))}
              </div>
            </section>

            <section className="space-y-4">
              <div className="flex justify-between items-center">
                <h2 className="text-xl font-bold">Recent Workouts</h2>
                <button className="text-primary text-sm font-medium hover:underline">View All</button>
              </div>
              <div className="space-y-4">
                <WorkoutItem 
                  title="Morning Yoga" 
                  type="Flexibility" 
                  duration="30 min" 
                  calories="120 kcal" 
                  time="08:00 AM" 
                />
                <WorkoutItem 
                  title="Upper Body Power" 
                  type="Strength" 
                  duration="55 min" 
                  calories="450 kcal" 
                  time="05:30 PM" 
                />
              </div>
            </section>

            {/* Smart AI Coach Section */}
            <SmartCoach />
          </div>

          {/* Sidebar Area */}
          <div className="space-y-8">
            <section className="glass rounded-[32px] p-8 text-center">
              <h2 className="text-xl font-bold mb-6">Daily Goals</h2>
              <div className="relative w-48 h-48 mx-auto mb-8">
                {/* SVG Progress Rings */}
                <svg className="w-full h-full transform -rotate-90">
                  <circle className="text-white/5" strokeWidth="12" stroke="currentColor" fill="transparent" r="80" cx="96" cy="96" />
                  <circle 
                    className="text-primary transition-all duration-1000 ease-out" 
                    strokeWidth="12" 
                    strokeDasharray={2 * Math.PI * 80} 
                    strokeDashoffset={2 * Math.PI * 80 * 0.2} 
                    strokeLinecap="round" 
                    stroke="currentColor" 
                    fill="transparent" 
                    r="80" cx="96" cy="96" 
                  />
                  
                  <circle className="text-white/5" strokeWidth="12" stroke="currentColor" fill="transparent" r="60" cx="96" cy="96" />
                  <circle 
                    className="text-secondary transition-all duration-1000 ease-out" 
                    strokeWidth="12" 
                    strokeDasharray={2 * Math.PI * 60} 
                    strokeDashoffset={2 * Math.PI * 60 * 0.4} 
                    strokeLinecap="round" 
                    stroke="currentColor" 
                    fill="transparent" 
                    r="60" cx="96" cy="96" 
                  />
                </svg>
                <div className="absolute inset-0 flex flex-col items-center justify-center">
                  <span className="text-3xl font-bold">80%</span>
                  <span className="text-[10px] text-white/40 uppercase tracking-widest">Total</span>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4 text-left">
                <div className="flex items-center gap-2">
                  <div className="w-2 h-2 rounded-full bg-primary" />
                  <span className="text-xs text-white/60">Move</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-2 h-2 rounded-full bg-secondary" />
                  <span className="text-xs text-white/60">Exercise</span>
                </div>
              </div>
            </section>

            <section className="glass rounded-[32px] p-6">
              <div className="flex items-center gap-4 mb-6">
                <div className="w-12 h-12 rounded-2xl bg-primary/10 flex items-center justify-center">
                  <Dumbbell className="w-6 h-6 text-primary" />
                </div>
                <div>
                  <h3 className="font-bold">Next Milestone</h3>
                  <p className="text-xs text-white/40">50 workouts challenge</p>
                </div>
              </div>
              <div className="w-full h-2 bg-white/5 rounded-full overflow-hidden">
                <div className="h-full bg-primary w-[75%]" />
              </div>
              <p className="mt-2 text-right text-xs font-medium text-white/40">38/50 Complete</p>
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}

function SmartCoach() {
  const [loading, setLoading] = React.useState(false);
  const [recommendation, setRecommendation] = React.useState<any>(null);
  const [currentSet, setCurrentSet] = React.useState(1);
  const [formData, setFormData] = React.useState({ weight: '', reps: '', feeling: '' });

  const getRecommendation = async () => {
    if (!formData.weight || !formData.reps) return;
    setLoading(true);
    try {
      const res = await fetch('http://localhost:8080/api/recommend', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          exercise_id: 1, 
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
      // フォールバック
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
    setFormData({ ...formData, feeling: '' }); // 感想だけリセット
  };

  return (
    <section className="glass rounded-[32px] p-8 border-primary/20 bg-primary/5 overflow-hidden relative">
      <div className="absolute top-0 right-0 p-8 text-primary/5">
        <BrainCircuit className="w-32 h-32" />
      </div>
      
      <div className="relative z-10">
        <div className="flex justify-between items-center mb-8">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-primary rounded-xl">
              <Sparkles className="w-5 h-5 text-black" />
            </div>
            <h2 className="text-xl font-bold italic uppercase tracking-wider">Smart Live Coach</h2>
          </div>
          <div className="px-4 py-1.5 glass rounded-full text-xs font-black text-primary">
            SET {currentSet}
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          {/* Input Form */}
          <div className="space-y-6">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-[10px] uppercase font-black text-white/40 ml-1">Weight (kg)</label>
                <input 
                  type="number" 
                  value={formData.weight}
                  onChange={(e) => setFormData({...formData, weight: e.target.value})}
                  className="w-full bg-white/5 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-xl font-bold"
                  placeholder="80"
                />
              </div>
              <div className="space-y-2">
                <label className="text-[10px] uppercase font-black text-white/40 ml-1">Reps</label>
                <input 
                  type="number" 
                  value={formData.reps}
                  onChange={(e) => setFormData({...formData, reps: e.target.value})}
                  className="w-full bg-white/5 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-secondary/50 text-xl font-bold"
                  placeholder="10"
                />
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-[10px] uppercase font-black text-white/40 ml-1">Set Feeling / Impression</label>
              <textarea 
                value={formData.feeling}
                onChange={(e) => setFormData({...formData, feeling: e.target.value})}
                className="w-full bg-white/5 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-accent/50 text-sm min-h-[100px]"
                placeholder="例: きつかった、まだ余裕がある、肩に違和感..."
              />
            </div>
            <button 
              onClick={getRecommendation}
              disabled={loading || !formData.weight || !formData.reps}
              className="w-full py-4 bg-primary text-black font-black rounded-2xl glow-primary hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-30 disabled:pointer-events-none uppercase tracking-widest flex items-center justify-center gap-2"
            >
              {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : "Set Finished - Analyze"}
            </button>
          </div>

          {/* AI Result Area */}
          <div className="min-h-[300px] flex items-center justify-center">
            <AnimatePresence mode="wait">
              {!recommendation ? (
                <motion.div 
                  initial={{ opacity: 0 }} 
                  animate={{ opacity: 1 }} 
                  exit={{ opacity: 0 }}
                  className="text-center space-y-4"
                >
                  <BrainCircuit className="w-16 h-16 text-white/10 mx-auto animate-float" />
                  <p className="text-sm text-white/30 font-medium">セット終了時に入力してください。<br/>AIが次のアクションを判断します。</p>
                </motion.div>
              ) : (
                <motion.div 
                  initial={{ opacity: 0, x: 20 }}
                  animate={{ opacity: 1, x: 0 }}
                  className="w-full space-y-6"
                >
                  <div className={`p-6 rounded-[24px] border-2 flex items-center justify-between ${
                    recommendation.next_action === 'STOP' 
                    ? 'bg-accent/10 border-accent/30 text-accent' 
                    : 'bg-primary/10 border-primary/30 text-primary'
                  }`}>
                    <div className="flex items-center gap-3">
                      {recommendation.next_action === 'STOP' ? <Flame className="w-6 h-6" /> : <TrendingUp className="w-6 h-6" />}
                      <span className="text-2xl font-black italic tracking-tighter uppercase">
                        {recommendation.next_action === 'STOP' ? 'STOP NOW' : 'NEXT SET: CONTINUE'}
                      </span>
                    </div>
                  </div>

                  <div className="glass p-6 rounded-[24px] border-white/5 space-y-4">
                    <div className="flex items-center gap-2 text-primary font-bold text-sm">
                      <Sparkles className="w-4 h-4" /> AI COACH SAYS
                    </div>
                    <p className="text-white/80 leading-relaxed font-medium">
                      {recommendation.recommendation}
                    </p>
                    {recommendation.next_action !== 'STOP' && (
                      <div className="flex gap-4 pt-2">
                        <div className="flex-1 bg-white/5 p-3 rounded-xl border border-white/5">
                          <span className="text-[9px] uppercase font-black text-white/30 block mb-1">Target Kg</span>
                          <span className="text-lg font-bold">{recommendation.target_weight}</span>
                        </div>
                        <div className="flex-1 bg-white/5 p-3 rounded-xl border border-white/5">
                          <span className="text-[9px] uppercase font-black text-white/30 block mb-1">Target Reps</span>
                          <span className="text-lg font-bold">{recommendation.target_reps}</span>
                        </div>
                      </div>
                    )}
                    <div className="pt-4 border-t border-white/5 flex justify-between items-end">
                      <div>
                        <p className="text-[10px] text-white/20 uppercase font-black mb-1">Coach's Rationale</p>
                        <p className="text-[11px] text-white/40 italic">{recommendation.reason}</p>
                      </div>
                      <div className="text-right">
                        <p className="text-[10px] text-white/20 uppercase font-black mb-1">Personal Best</p>
                        <p className="text-xs font-bold text-white/60">{recommendation.max_weight}<span className="text-[10px] ml-0.5">kg</span></p>
                      </div>
                    </div>
                  </div>

                  {recommendation.next_action !== 'STOP' ? (
                    <button 
                      onClick={handleNextSet}
                      className="w-full py-3 bg-white/10 hover:bg-white/20 text-white font-bold rounded-2xl transition-all"
                    >
                      OK, Go to Next Set
                    </button>
                  ) : (
                    <button 
                      onClick={() => {setRecommendation(null); setCurrentSet(1); setFormData({weight:'', reps:'', feeling:''})}}
                      className="w-full py-3 bg-accent/20 hover:bg-accent/30 text-accent font-bold rounded-2xl transition-all"
                    >
                      Finish Exercise
                    </button>
                  ) }
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </div>
      </div>
    </section>
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
