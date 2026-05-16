"use client";

import React, { useState, useEffect } from "react";
import { Search, Plus, Dumbbell, X, Loader2, Activity } from "lucide-react";

interface Exercise {
  id: string;
  name: string;
  category?: string;
  equipment?: string;
  primaryMuscles?: string[];
}

const MUSCLE_CATEGORIES: Record<string, string[]> = {
  "胸": ["大胸筋"],
  "背中": ["広背筋", "背中中部", "下背部", "僧帽筋"],
  "肩": ["肩"],
  "腕": ["上腕二頭筋", "上腕三頭筋", "前腕"],
  "脚": ["大腿四頭筋", "ハムストリングス", "大臀筋", "ふくらはぎ"],
  "腹/体幹": ["腹筋"],
  "その他": ["首"]
};

const EQUIPMENTS = [
  "自重", "ダンベル", "バーベル", "マシン", "ケーブル", "バンド", "ケトルベル", "メディシンボール", "バランスボール", "その他"
];

const EXERCISE_LABELS: Record<string, string> = {
  strength: "筋力トレーニング",
  stretching: "ストレッチ",
  cardio: "有酸素",
  plyometrics: "瞬発系",
  powerlifting: "パワーリフティング",
  olympic_weightlifting: "重量挙げ",
  strongman: "ストロングマン",
};

function displayExerciseLabel(value?: string) {
  if (!value) return "";
  return EXERCISE_LABELS[value] || value;
}

interface Props {
  onClose: () => void;
  onSelect: (ex: { id: string, name: string }) => void;
}

export function ExerciseSelectorModal({ onClose, onSelect }: Props) {
  const [exercises, setExercises] = useState<Exercise[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [broadMuscle, setBroadMuscle] = useState("");
  const [muscleFilter, setMuscleFilter] = useState("");
  const [equipFilter, setEquipFilter] = useState("");
  const [isCustomizing, setIsCustomizing] = useState(false);
  
  // カスタム種目用
  const [customName, setCustomName] = useState("");
  const [customCategory, setCustomCategory] = useState("筋力トレーニング");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const apiBase = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

  useEffect(() => {
    const fetch_ = async () => {
      const params = new URLSearchParams();
      if (searchQuery.trim()) params.set('name', searchQuery.trim());
      // 細かい筋肉が指定されていればそちらを優先、なければざっくりカテゴリで絞る
      const effectiveMuscle = muscleFilter || (broadMuscle ? (MUSCLE_CATEGORIES[broadMuscle]?.[0] ?? '') : '');
      if (effectiveMuscle) params.set('muscle', effectiveMuscle);
      if (equipFilter) params.set('equipment', equipFilter);
      const qs = params.toString();
      const res = await fetch(`${apiBase}/api/exercises${qs ? '?' + qs : ''}`);
      const data = await res.json();
      setExercises(data || []);
    };
    const timer = setTimeout(fetch_, 300);
    return () => clearTimeout(timer);
  }, [searchQuery, muscleFilter, broadMuscle, equipFilter, apiBase]);

  const handleBroadMuscleChange = (broad: string) => {
    setBroadMuscle(broad);
    setMuscleFilter(""); // 親カテゴリが変わったら細かい部位はリセット
  };

  const handleDetailedMuscleChange = (detailed: string) => {
    setMuscleFilter(detailed);
    if (detailed) {
      // 選ばれた細かい筋肉が含まれる親カテゴリを自動選択
      for (const [broad, detailedList] of Object.entries(MUSCLE_CATEGORIES)) {
        if (detailedList.includes(detailed)) {
          setBroadMuscle(broad);
          break;
        }
      }
    }
  };

  const availableDetailedMuscles = broadMuscle 
    ? MUSCLE_CATEGORIES[broadMuscle] 
    : Object.values(MUSCLE_CATEGORIES).flat();

  // サーバー側で検索済みのため、exercises をそのまま表示する
  const filtered = exercises;

  const handleCustomSubmit = async () => {
    if (!customName.trim()) return;
    setIsSubmitting(true);
    try {
      const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/exercises/custom`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ 
          name: customName, 
          category: customCategory, 
          equipment: "その他", 
          primary_muscle: "" 
        })
      });
      if (res.ok) {
        const data = await res.json();
        onSelect({ id: data.id, name: data.name });
      } else {
        alert("追加に失敗しました。");
      }
    } catch (e) {
      console.error(e);
      alert("通信エラーが発生しました。");
    }
    setIsSubmitting(false);
  };

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/80 backdrop-blur-md" onClick={onClose}>
      <div className="bg-[#111] border border-white/10 rounded-3xl shadow-2xl max-w-lg w-full max-h-[85vh] overflow-hidden flex flex-col" onClick={e => e.stopPropagation()}>
        
        <div className="p-5 border-b border-white/10 flex justify-between items-center bg-[#1a1a1a]">
          <h2 className="text-xl font-bold text-white flex items-center gap-2">
            <Dumbbell className="text-primary w-5 h-5" />
            種目を選択
          </h2>
          <button onClick={onClose} className="text-white/50 hover:text-white p-2 rounded-full hover:bg-white/10 transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        {isCustomizing ? (
          <div className="p-6 overflow-y-auto space-y-5 flex-1 text-white">
            <h3 className="font-bold text-lg text-primary mb-2">新しい種目を追加</h3>
            <p className="text-sm text-white/50 mb-4">
              見つからない種目は自分で追加できます。追加した種目は辞書に保存され、後からいつでも記録に使用できます。
            </p>

            <div className="space-y-2">
              <label className="text-xs font-black text-white/40">種目名 <span className="text-red-500">*</span></label>
              <input 
                type="text" 
                value={customName}
                onChange={e => setCustomName(e.target.value)}
                placeholder="例: ダンベルプレス"
                className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-black text-white/40">カテゴリ（任意）</label>
              <select 
                value={customCategory}
                onChange={e => setCustomCategory(e.target.value)}
                className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
              >
                <option value="筋力トレーニング">筋力トレーニング</option>
                <option value="有酸素運動">有酸素運動</option>
                <option value="ストレッチ">ストレッチ</option>
                <option value="その他">その他</option>
              </select>
            </div>

            <div className="pt-4 flex gap-3">
              <button 
                onClick={() => setIsCustomizing(false)}
                className="flex-1 py-3 bg-white/10 text-white font-bold rounded-xl hover:bg-white/20 transition-colors"
              >
                キャンセル
              </button>
              <button 
                onClick={handleCustomSubmit}
                disabled={isSubmitting || !customName.trim()}
                className="flex-1 py-3 bg-primary text-black font-bold rounded-xl hover:scale-[1.02] transition-all disabled:opacity-50 flex justify-center items-center gap-2"
              >
                {isSubmitting ? <Loader2 className="w-5 h-5 animate-spin" /> : "追加して選択"}
              </button>
            </div>
          </div>
        ) : (
          <div className="flex flex-col flex-1 overflow-hidden">
            <div className="p-4 border-b border-white/5 space-y-3">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40 w-4 h-4" />
                <input 
                  type="text" 
                  placeholder="種目名で検索..." 
                  value={searchQuery}
                  onChange={e => setSearchQuery(e.target.value)}
                  className="w-full bg-white/5 border border-white/10 rounded-lg pl-10 pr-4 py-2 focus:outline-none focus:border-primary/50 text-white"
                />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <select 
                  value={broadMuscle}
                  onChange={e => handleBroadMuscleChange(e.target.value)}
                  className="bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white/80 focus:outline-none focus:border-primary/50 appearance-none"
                >
                  <option value="">大まかな部位(すべて)</option>
                  {Object.keys(MUSCLE_CATEGORIES).map(m => <option key={m} value={m}>{m}</option>)}
                </select>
                <select 
                  value={muscleFilter}
                  onChange={e => handleDetailedMuscleChange(e.target.value)}
                  className="bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white/80 focus:outline-none focus:border-primary/50 appearance-none"
                >
                  <option value="">細かい筋肉(すべて)</option>
                  {availableDetailedMuscles.map(m => <option key={m} value={m}>{m}</option>)}
                </select>
              </div>
              <div>
                <select 
                  value={equipFilter}
                  onChange={e => setEquipFilter(e.target.value)}
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white/80 focus:outline-none focus:border-primary/50 appearance-none"
                >
                  <option value="">すべての器具</option>
                  {EQUIPMENTS.map(eq => <option key={eq} value={eq}>{eq}</option>)}
                </select>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto p-2 space-y-1">
              {filtered.slice(0, 50).map(ex => (
                <button 
                  key={ex.id}
                  onClick={() => onSelect({ id: ex.id, name: ex.name })}
                  className="w-full text-left px-4 py-3 hover:bg-white/5 rounded-xl transition-colors flex flex-col group"
                >
                  <div className="flex justify-between items-center w-full">
                    <span className="text-white font-medium group-hover:text-primary transition-colors">{ex.name}</span>
                    {ex.category && <span className="text-[10px] px-2 py-0.5 bg-white/5 rounded-full text-white/40">{displayExerciseLabel(ex.category)}</span>}
                  </div>
                  <div className="flex gap-2 mt-1">
                    {ex.primaryMuscles && ex.primaryMuscles.length > 0 && (
                      <span className="text-[10px] text-white/30 flex items-center gap-1">
                        <Activity className="w-3 h-3" /> {ex.primaryMuscles[0]}
                      </span>
                    )}
                    {ex.equipment && (
                      <span className="text-[10px] text-white/30 flex items-center gap-1">
                        <Dumbbell className="w-3 h-3" /> {ex.equipment}
                      </span>
                    )}
                  </div>
                </button>
              ))}
              
              {filtered.length === 0 && (
                <div className="text-center py-10 text-white/40">
                  <p>種目が見つかりません</p>
                </div>
              )}
            </div>

            <div className="p-4 border-t border-white/10 bg-[#1a1a1a]">
              <button 
                onClick={() => setIsCustomizing(true)}
                className="w-full py-3 bg-white/5 hover:bg-white/10 border border-white/10 border-dashed text-white font-bold rounded-xl transition-all flex justify-center items-center gap-2"
              >
                <Plus className="w-5 h-5 text-primary" />
                見つからない種目を新しく追加
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
