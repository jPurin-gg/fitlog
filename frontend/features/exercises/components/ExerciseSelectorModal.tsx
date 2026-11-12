"use client";

import React, { useState, useEffect } from "react";
import { Search, Plus, Dumbbell, X, Loader2, Activity, Star } from "lucide-react";
import { useBodyScrollLock } from "@/hooks/useBodyScrollLock";
import { apiFetch } from "@/lib/api";
import { useExerciseFilterOptions } from "../useExerciseFilterOptions";
import { useExerciseSearch } from "../useExerciseSearch";

interface Exercise {
  id: string;
  name: string;
  category?: string | null;
  equipment?: string | null;
  primaryMuscles?: string[] | null;
}

const EXERCISE_LABELS: Record<string, string> = {
  strength: "筋力トレーニング",
  stretching: "ストレッチ",
  cardio: "有酸素",
  plyometrics: "瞬発系",
  powerlifting: "パワーリフティング",
  olympic_weightlifting: "重量挙げ",
  strongman: "ストロングマン",
};

function displayExerciseLabel(value?: string | null) {
  if (!value) return "";
  return EXERCISE_LABELS[value] || value;
}

interface Props {
  onClose: () => void;
  onSelect: (ex: { id: string, name: string }) => void;
}

export function ExerciseSelectorModal({ onClose, onSelect }: Props) {
  const [favoriteExercises, setFavoriteExercises] = useState<Exercise[]>([]);
  const [recentExercises, setRecentExercises] = useState<Exercise[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [broadMuscle, setBroadMuscle] = useState("");
  const [muscleFilter, setMuscleFilter] = useState("");
  const [equipFilter, setEquipFilter] = useState("");
  const [isCustomizing, setIsCustomizing] = useState(false);
  const [activeTab, setActiveTab] = useState<"favorite" | "recent" | "search">("favorite");
  // 検索タブを開くまでは一覧を取得しない。一度開いた後は、タブを行き来しても結果を保つ。
  const [searchOpened, setSearchOpened] = useState(false);

  // カスタム種目用
  const [customName, setCustomName] = useState("");
  const [customCategory, setCustomCategory] = useState("筋力トレーニング");
  const [isSubmitting, setIsSubmitting] = useState(false);

  useBodyScrollLock();

  const filterOptions = useExerciseFilterOptions();
  const muscleCategories = filterOptions.muscleCategories;
  const broadMuscles = muscleCategories.find(category => category.name === broadMuscle)?.muscles ?? [];
  // 細かい筋肉が指定されていればそれだけで、なければ大分類に属する全部位で絞る
  const search = useExerciseSearch(
    {
      name: searchQuery,
      muscles: muscleFilter ? [muscleFilter] : broadMuscles,
      equipment: equipFilter ? [equipFilter] : [],
      level: "",
    },
    searchOpened,
  );

  const fetchFavorites = React.useCallback(async () => {
    try {
      const data = await apiFetch<Exercise[]>("/api/exercises/favorites");
      setFavoriteExercises(Array.isArray(data) ? data : []);
    } catch (e) {
      console.error("Failed to fetch favorite exercises:", e);
      setFavoriteExercises([]);
    }
  }, []);

  useEffect(() => {
    fetchFavorites();
  }, [fetchFavorites]);

  useEffect(() => {
    const fetchRecent = async () => {
      try {
        const data = await apiFetch<Exercise[]>("/api/exercises/recent");
        setRecentExercises(Array.isArray(data) ? data : []);
      } catch (e) {
        console.error("Failed to fetch recent exercises:", e);
        setRecentExercises([]);
      }
    };
    fetchRecent();
  }, []);

  const openSearchTab = () => {
    setActiveTab("search");
    setSearchOpened(true);
  };

  const handleBroadMuscleChange = (broad: string) => {
    setBroadMuscle(broad);
    setMuscleFilter(""); // 親カテゴリが変わったら細かい部位はリセット
  };

  const handleDetailedMuscleChange = (detailed: string) => {
    setMuscleFilter(detailed);
    if (detailed) {
      // 選ばれた細かい筋肉が含まれる親カテゴリを自動選択
      const parent = muscleCategories.find(category => category.muscles.includes(detailed));
      if (parent) setBroadMuscle(parent.name);
    }
  };

  const availableDetailedMuscles = broadMuscle
    ? broadMuscles
    : muscleCategories.flatMap(category => category.muscles);

  const favoriteIds = React.useMemo(() => new Set(favoriteExercises.map(ex => ex.id)), [favoriteExercises]);
  const isSearchTab = activeTab === "search";
  const listed: Exercise[] = activeTab === "favorite" ? favoriteExercises : activeTab === "recent" ? recentExercises : search.items;

  const toggleFavorite = async (exercise: Exercise) => {
    const favorite = favoriteIds.has(exercise.id);
    try {
      await apiFetch<void>(`/api/exercises/${encodeURIComponent(exercise.id)}/favorite`, {
        method: favorite ? "DELETE" : "PUT",
      });
      if (favorite) {
        setFavoriteExercises(prev => prev.filter(ex => ex.id !== exercise.id));
      } else {
        setFavoriteExercises(prev => [exercise, ...prev.filter(ex => ex.id !== exercise.id)]);
      }
    } catch (e) {
      console.error(e);
      alert("お気に入りの更新に失敗しました。");
    }
  };

  const handleCustomSubmit = async () => {
    if (!customName.trim()) return;
    setIsSubmitting(true);
    try {
      const data = await apiFetch<Exercise>("/api/exercises", {
        method: "POST",
        body: JSON.stringify({
          name: customName,
          category: customCategory,
          equipment: "その他",
          primary_muscle: ""
        })
      });
      onSelect({ id: data.id, name: data.name });
    } catch (e) {
      console.error(e);
      alert("通信エラーが発生しました。");
    }
    setIsSubmitting(false);
  };

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/80 backdrop-blur-md overflow-hidden overscroll-none touch-none" onClick={onClose}>
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
              <div className="grid grid-cols-3 gap-2 rounded-2xl bg-black/30 p-1 border border-white/10">
                <button
                  type="button"
                  onClick={() => setActiveTab("favorite")}
                  className={`py-2 rounded-xl text-sm font-bold transition-colors ${
                    activeTab === "favorite" ? "bg-primary text-black" : "text-white/50 hover:text-white hover:bg-white/5"
                  }`}
                >
                  お気に入り
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab("recent")}
                  className={`py-2 rounded-xl text-sm font-bold transition-colors ${
                    activeTab === "recent" ? "bg-primary text-black" : "text-white/50 hover:text-white hover:bg-white/5"
                  }`}
                >
                  最近
                </button>
                <button
                  type="button"
                  onClick={openSearchTab}
                  className={`py-2 rounded-xl text-sm font-bold transition-colors ${
                    activeTab === "search" ? "bg-primary text-black" : "text-white/50 hover:text-white hover:bg-white/5"
                  }`}
                >
                  検索
                </button>
              </div>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40 w-4 h-4" />
                <input
                  type="text"
                  placeholder="種目名で検索..."
                  value={searchQuery}
                  onFocus={openSearchTab}
                  onChange={e => {
                    setSearchQuery(e.target.value);
                    openSearchTab();
                  }}
                  className="w-full bg-white/5 border border-white/10 rounded-lg pl-10 pr-4 py-2 focus:outline-none focus:border-primary/50 text-white"
                />
              </div>
              {/* 部位・器具の絞り込みは検索結果だけに効くため、検索タブでのみ表示する */}
              {isSearchTab && filterOptions.status === "error" && (
                <div role="alert" className="flex items-center justify-between gap-3 rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-200">
                  <span>部位・器具の選択肢を取得できませんでした。</span>
                  <button type="button" onClick={filterOptions.retry} className="shrink-0 font-bold underline hover:text-white">
                    選択肢を再取得
                  </button>
                </div>
              )}
              {isSearchTab && filterOptions.status !== "error" && (
                <>
                  <div className="grid grid-cols-2 gap-2">
                    <select
                      aria-label="大まかな部位"
                      value={broadMuscle}
                      onChange={e => handleBroadMuscleChange(e.target.value)}
                      className="bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white/80 focus:outline-none focus:border-primary/50 appearance-none"
                    >
                      <option value="">大まかな部位(すべて)</option>
                      {muscleCategories.map(category => <option key={category.name} value={category.name}>{category.name}</option>)}
                    </select>
                    <select
                      aria-label="細かい筋肉"
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
                      aria-label="器具"
                      value={equipFilter}
                      onChange={e => setEquipFilter(e.target.value)}
                      className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white/80 focus:outline-none focus:border-primary/50 appearance-none"
                    >
                      <option value="">すべての器具</option>
                      {filterOptions.equipment.map(eq => <option key={eq} value={eq}>{eq}</option>)}
                    </select>
                  </div>
                </>
              )}
            </div>

            <div className="flex-1 overflow-y-auto p-2 space-y-1">
              {isSearchTab && search.status === "loading" ? (
                <div role="status" className="min-h-52 flex flex-col items-center justify-center gap-3 text-white/40">
                  <Loader2 className="w-6 h-6 animate-spin" />
                  <p className="text-sm">種目を読み込み中...</p>
                </div>
              ) : isSearchTab && search.status === "error" ? (
                <div role="alert" className="min-h-52 flex flex-col items-center justify-center gap-4 text-center px-8">
                  <p className="text-sm text-red-200 leading-relaxed">{search.error}</p>
                  <button
                    type="button"
                    onClick={search.reload}
                    className="px-5 py-2 bg-white/10 hover:bg-white/20 text-white text-sm font-bold rounded-xl transition-colors"
                  >
                    再試行
                  </button>
                </div>
              ) : listed.length === 0 && activeTab === "favorite" ? (
                <div className="min-h-52 flex flex-col items-center justify-center text-center text-white/40 px-8">
                  <Star className="w-8 h-8 text-primary mb-3" />
                  <p className="text-sm leading-relaxed">
                    まだお気に入りがありません。検索タブでよく使う種目に星を付けると、ここからすぐ選べます。
                  </p>
                </div>
              ) : listed.length === 0 && activeTab === "recent" ? (
                <div className="min-h-52 flex flex-col items-center justify-center text-center text-white/40 px-8">
                  <Dumbbell className="w-8 h-8 text-primary mb-3" />
                  <p className="text-sm leading-relaxed">
                    まだ最近使った種目がありません。記録が増えると、ここからすぐ選べます。
                  </p>
                </div>
              ) : listed.length === 0 ? (
                <div className="text-center py-10 text-white/40">
                  <p>種目が見つかりません</p>
                </div>
              ) : listed.map(ex => (
                <div
                  key={ex.id}
                  className="w-full px-3 py-3 hover:bg-white/5 rounded-xl transition-colors flex items-start gap-3 group"
                >
                  <button
                    type="button"
                    onClick={() => toggleFavorite(ex)}
                    className={`mt-0.5 p-2 rounded-xl transition-colors ${
                      favoriteIds.has(ex.id)
                        ? "text-primary bg-primary/10 hover:bg-primary/20"
                        : "text-white/25 hover:text-primary hover:bg-white/10"
                    }`}
                    aria-label={favoriteIds.has(ex.id) ? "お気に入りから外す" : "お気に入りに追加"}
                  >
                    <Star className={`w-4 h-4 ${favoriteIds.has(ex.id) ? "fill-current" : ""}`} />
                  </button>
                  <button
                    type="button"
                    onClick={() => onSelect({ id: ex.id, name: ex.name })}
                    className="flex-1 min-w-0 text-left"
                  >
                    <div className="flex justify-between items-center gap-3 w-full">
                      <span className="text-white font-medium group-hover:text-primary transition-colors">{ex.name}</span>
                      {ex.category && <span className="shrink-0 text-[10px] px-2 py-0.5 bg-white/5 rounded-full text-white/40">{displayExerciseLabel(ex.category)}</span>}
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
                </div>
              ))}

              {/* 一覧は1ページずつ取得するため、残りがあることと続きの取得手段を示す */}
              {isSearchTab && search.status === "ready" && search.items.length > 0 && (
                <div className="px-3 py-3 space-y-3 text-center">
                  <p className="text-xs text-white/40">{search.total}件中 {search.items.length}件を表示</p>
                  {search.loadMoreError && (
                    <p role="alert" className="text-xs text-red-200">{search.loadMoreError}</p>
                  )}
                  {search.hasMore && (
                    <button
                      type="button"
                      onClick={search.loadMore}
                      disabled={search.loadingMore}
                      className="w-full py-2 bg-white/5 hover:bg-white/10 border border-white/10 text-white/80 text-sm font-bold rounded-xl transition-colors disabled:opacity-50 flex justify-center items-center gap-2"
                    >
                      {search.loadingMore && <Loader2 className="w-4 h-4 animate-spin" />}
                      {search.loadMoreError ? "続きを再試行" : "さらに表示"}
                    </button>
                  )}
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
