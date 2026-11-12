"use client"

import React, { useState, useEffect } from 'react'
import { Search, Filter, Dumbbell, Activity, Shield, Plus, Loader2 } from 'lucide-react'
import { apiErrorMessage, apiFetch } from '@/lib/api'
import { getExercise } from './api'
import type { Exercise, ExerciseSummary } from './types'
import { useExerciseFilterOptions } from './useExerciseFilterOptions'
import { useExerciseSearch } from './useExerciseSearch'

const CATEGORY_OPTIONS = [
  { value: "strength", label: "筋力トレーニング" },
  { value: "cardio", label: "有酸素" },
  { value: "stretching", label: "ストレッチ" },
  { value: "plyometrics", label: "瞬発系" },
  { value: "その他", label: "その他" },
]

const DIFFICULTY_OPTIONS = [
  { value: "すべて", label: "すべて" },
  { value: "初級", label: "初級" },
  { value: "中級", label: "中級" },
  { value: "上級", label: "上級" },
]

const EXERCISE_LABELS: Record<string, string> = {
  beginner: "初級",
  intermediate: "中級",
  expert: "上級",
  compound: "複合",
  isolation: "単関節",
  strength: "筋力トレーニング",
  stretching: "ストレッチ",
  cardio: "有酸素",
  plyometrics: "瞬発系",
  powerlifting: "パワーリフティング",
  olympic_weightlifting: "重量挙げ",
  strongman: "ストロングマン",
  push: "押す",
  pull: "引く",
  static: "静止",
}

function displayExerciseLabel(value?: string | null) {
  if (!value) return ""
  return EXERCISE_LABELS[value] || value
}

// 選択肢の取得前や取得失敗時でも、フォームの現在値を選択できる状態に保つ
function withCurrent(options: string[], current: string) {
  return options.includes(current) ? options : [current, ...options]
}

const DETAIL_ERROR_MESSAGE = '種目の詳細を取得できませんでした。通信状況を確認して、もう一度お試しください。'

function useExerciseDetail(exerciseID: string | undefined) {
  const [state, setState] = useState<{ id: string; exercise: Exercise | null; error: string }>({ id: '', exercise: null, error: '' })
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    if (!exerciseID) return
    const controller = new AbortController()
    setState({ id: exerciseID, exercise: null, error: '' })
    getExercise(exerciseID, controller.signal)
      .then(exercise => {
        if (!controller.signal.aborted) setState({ id: exerciseID, exercise, error: '' })
      })
      .catch(error => {
        if (!controller.signal.aborted) setState({ id: exerciseID, exercise: null, error: apiErrorMessage(error, DETAIL_ERROR_MESSAGE) })
      })
    return () => controller.abort()
  }, [exerciseID, reloadToken])

  // 別の種目を開いた直後に、前の種目の詳細を表示しない
  const current = state.id === exerciseID
  return {
    exercise: current ? state.exercise : null,
    error: current ? state.error : '',
    retry: () => setReloadToken(token => token + 1),
  }
}

export default function ExercisesPage() {
  const [muscle, setMuscle] = useState('すべて')
  const [difficulty, setDifficulty] = useState('すべて')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedEquipments, setSelectedEquipments] = useState<string[]>([])
  const [selectedExercise, setSelectedExercise] = useState<ExerciseSummary | null>(null)
  const [showAddModal, setShowAddModal] = useState(false)
  const [isAdding, setIsAdding] = useState(false)
  const [addError, setAddError] = useState('')
  // 器具・部位の選択肢は、種目カタログと同じ表記でAPIが返す
  const filterOptions = useExerciseFilterOptions()
  const muscleOptions = filterOptions.muscleCategories.flatMap(category => category.muscles)
  const search = useExerciseSearch({
    name: searchQuery,
    muscles: muscle === 'すべて' ? [] : [muscle],
    equipment: selectedEquipments,
    level: difficulty === 'すべて' ? '' : difficulty,
  })
  const exercises = search.items
  // 手順文は一覧に含まれないため、詳細を開いたときに取得する
  const detail = useExerciseDetail(selectedExercise?.id)
  const [newExercise, setNewExercise] = useState({
    name: '',
    primaryMuscle: '大胸筋',
    secondaryMuscles: '',
    equipment: 'その他',
    category: 'strength',
    level: '初級',
    instructions: '',
  })

  const toggleEquipment = (eq: string) => {
    if (selectedEquipments.includes(eq)) {
      setSelectedEquipments(selectedEquipments.filter(e => e !== eq))
    } else {
      setSelectedEquipments([...selectedEquipments, eq])
    }
  }

  // モーダルが開いている間は背景のスクロールを無効化する（UX改善）
  useEffect(() => {
    if (selectedExercise) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = 'unset'
    }
    return () => {
      document.body.style.overflow = 'unset'
    }
  }, [selectedExercise])

  const resetAddForm = () => {
    setNewExercise({
      name: '',
      primaryMuscle: '大胸筋',
      secondaryMuscles: '',
      equipment: 'その他',
      category: 'strength',
      level: '初級',
      instructions: '',
    })
    setAddError('')
  }

  const closeAddModal = () => {
    setShowAddModal(false)
    resetAddForm()
  }

  const addExercise = async () => {
    if (!newExercise.name.trim()) {
      setAddError('種目名を入力してください。')
      return
    }
    setIsAdding(true)
    setAddError('')
    try {
      const secondaryMuscles = newExercise.secondaryMuscles
        .split(',')
        .map(v => v.trim())
        .filter(Boolean)
      const instructions = newExercise.instructions
        .split('\n')
        .map(v => v.trim())
        .filter(Boolean)
      const created = await apiFetch<Exercise>('/api/exercises', {
        method: 'POST',
        body: JSON.stringify({
          name: newExercise.name.trim(),
          category: newExercise.category,
          equipment: newExercise.equipment,
          level: newExercise.level,
          primary_muscles: newExercise.primaryMuscle ? [newExercise.primaryMuscle] : [],
          secondary_muscles: secondaryMuscles,
          instructions,
        })
      })
      setSelectedExercise(created)
      search.reload()
      closeAddModal()
    } catch (e) {
      console.error(e)
      setAddError(apiErrorMessage(e, '通信エラーが発生しました。'))
    } finally {
      setIsAdding(false)
    }
  }

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white pb-20 selection:bg-primary/30">
      {/* プレミアムなヘッダー */}
      <div className="bg-gradient-to-r from-blue-900 to-indigo-900 text-white pt-16 pb-20 px-4 shadow-lg rounded-b-3xl relative overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-full bg-[url('https://www.transparenttextures.com/patterns/cubes.png')] opacity-10"></div>
        <div className="max-w-5xl mx-auto relative z-10 flex flex-col items-center text-center">
          <Dumbbell className="w-16 h-16 mb-4 text-blue-300 opacity-90" />
          <h1 className="text-4xl md:text-5xl font-extrabold tracking-tight mb-4 text-transparent bg-clip-text bg-gradient-to-r from-white to-blue-200">
            種目辞書
          </h1>
          <p className="text-blue-200 text-lg max-w-xl">
            約900種目以上のトレーニングデータベース。部位や器具から、あなたに最適な種目を見つけましょう。
          </p>
        </div>
      </div>

      <div className="max-w-5xl mx-auto px-4 -mt-10 relative z-20">
        {/* 検索・フィルターバー */}
        <div className="bg-[#111] border border-white/10 rounded-2xl shadow-xl p-4 md:p-6 mb-8 flex flex-col gap-4">
          <div className="relative w-full">
            <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-white/40 w-5 h-5" />
            <input 
              type="text" 
              placeholder="種目名や器具で検索..." 
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-12 pr-4 py-3 text-white font-medium bg-black/40 border border-white/10 rounded-xl focus:outline-none focus:border-primary/50 transition-all"
            />
          </div>
          <button
            onClick={() => setShowAddModal(true)}
            className="w-full sm:w-auto sm:self-start px-5 py-3 bg-primary text-black font-black rounded-xl hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center justify-center gap-2"
          >
            <Plus className="w-5 h-5" />
            種目を追加
          </button>
          
          <div className="flex overflow-x-auto pb-2 gap-2 hide-scrollbar">
            <div className="flex items-center text-white/40 text-xs font-bold whitespace-nowrap mr-2"><Activity className="w-4 h-4 mr-1"/> 部位:</div>
            {['すべて', ...muscleOptions].map(m => (
              <button
                key={m}
                onClick={() => setMuscle(m)}
                className={`whitespace-nowrap px-5 py-2 text-sm rounded-xl font-medium transition-all border flex-shrink-0 ${
                  muscle === m 
                  ? 'bg-primary text-black border-primary font-bold shadow-[0_0_15px_rgba(255,170,0,0.3)]' 
                  : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10 hover:text-white'
                }`}
              >
                {m}
              </button>
            ))}
          </div>

          <div className="flex overflow-x-auto pb-2 gap-2 hide-scrollbar">
            <div className="flex items-center text-white/40 text-xs font-bold whitespace-nowrap mr-2"><Shield className="w-4 h-4 mr-1"/> 難易度:</div>
            {DIFFICULTY_OPTIONS.map(option => (
              <button
                key={option.value}
                onClick={() => setDifficulty(option.value)}
                className={`whitespace-nowrap px-5 py-2 text-sm rounded-xl font-medium transition-all border flex-shrink-0 ${
                  difficulty === option.value
                  ? 'bg-secondary text-black border-secondary font-bold shadow-[0_0_15px_rgba(0,210,255,0.25)]'
                  : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10 hover:text-white'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>

          <div className="flex overflow-x-auto pb-2 gap-2 hide-scrollbar">
            <div className="flex items-center text-white/40 text-xs font-bold whitespace-nowrap mr-2"><Dumbbell className="w-4 h-4 mr-1"/> 器具:</div>
            {filterOptions.equipment.map(eq => (
              <button
                key={eq}
                onClick={() => toggleEquipment(eq)}
                className={`whitespace-nowrap px-4 py-2 text-sm rounded-xl font-medium transition-all border flex-shrink-0 ${
                  selectedEquipments.includes(eq)
                  ? 'bg-accent text-white border-accent font-bold shadow-[0_0_15px_rgba(255,50,50,0.3)]'
                  : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10 hover:text-white'
                }`}
              >
                {eq}
              </button>
            ))}
          </div>

          {filterOptions.status === 'error' && (
            <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200">
              <span>部位・器具の選択肢を取得できませんでした。</span>
              <button onClick={filterOptions.retry} className="font-bold underline hover:text-white">
                選択肢を再取得
              </button>
            </div>
          )}
        </div>

        {/* 種目リスト */}
        {search.status === 'loading' ? (
          <div role="status" aria-label="種目を読み込み中" className="flex justify-center items-center py-20">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
          </div>
        ) : search.status === 'error' ? (
          <div role="alert" className="text-center py-20 bg-[#111] rounded-3xl shadow-sm border border-red-500/30">
            <Activity className="w-16 h-16 mx-auto text-red-300/40 mb-4" />
            <h3 className="text-xl font-semibold text-white/80 mb-2">種目を読み込めませんでした</h3>
            <p className="text-red-200/80 mb-6 px-6">{search.error}</p>
            <button
              onClick={search.reload}
              className="px-6 py-3 bg-primary text-black font-black rounded-xl hover:scale-[1.02] active:scale-[0.98] transition-all"
            >
              再試行
            </button>
          </div>
        ) : exercises.length === 0 ? (
          <div className="text-center py-20 bg-[#111] rounded-3xl shadow-sm border border-white/10">
            <Activity className="w-16 h-16 mx-auto text-white/20 mb-4" />
            <h3 className="text-xl font-semibold text-white/80 mb-2">見つかりませんでした</h3>
            <p className="text-white/40">検索条件を変えてみてください。</p>
          </div>
        ) : (
          <>
            <div className="sticky top-0 z-40 py-4 mb-6 flex flex-col md:flex-row items-start md:items-center justify-between text-white/60 gap-4 bg-[#0a0a0a]/90 backdrop-blur-xl border-b border-white/5">
              <span className="font-medium bg-[#111] px-4 py-2 rounded-lg border border-white/10 flex-shrink-0">
                <span className="text-primary font-bold text-lg mr-1">{search.total}</span> 件の種目
                {search.hasMore && <span className="ml-2 text-sm text-white/40">（{exercises.length} 件を表示中）</span>}
              </span>

              {/* アクティブな検索条件の表示 */}
              <div className="flex flex-wrap items-center gap-2 bg-[#111] px-4 py-2 rounded-lg border border-white/10">
                <Filter className="w-4 h-4 text-white/40 mr-1" />
                <span className="text-sm text-white/50 mr-2">現在の条件:</span>
                
                {muscle !== 'すべて' && (
                  <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-primary/20 text-primary border border-primary/30">
                    部位: {muscle}
                    <button onClick={() => setMuscle('すべて')} className="ml-1.5 text-primary hover:text-white font-bold text-sm focus:outline-none leading-none">
                      &times;
                    </button>
                  </span>
                )}
                
                {selectedEquipments.length > 0 && (
                  <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-accent/20 text-accent border border-accent/30">
                    器具: {selectedEquipments.join(', ')}
                    <button onClick={() => setSelectedEquipments([])} className="ml-1.5 text-accent hover:text-white font-bold text-sm focus:outline-none leading-none">
                      &times;
                    </button>
                  </span>
                )}

                {difficulty !== 'すべて' && (
                  <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-secondary/20 text-secondary border border-secondary/30">
                    難易度: {displayExerciseLabel(difficulty)}
                    <button onClick={() => setDifficulty('すべて')} className="ml-1.5 text-secondary hover:text-white font-bold text-sm focus:outline-none leading-none">
                      &times;
                    </button>
                  </span>
                )}
                
                {searchQuery && (
                  <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-white/10 text-white/80 border border-white/20">
                    キーワード: &quot;{searchQuery}&quot;
                    <button onClick={() => setSearchQuery('')} className="ml-1.5 text-white/60 hover:text-white font-bold text-sm focus:outline-none leading-none">
                      &times;
                    </button>
                  </span>
                )}

                {muscle === 'すべて' && difficulty === 'すべて' && selectedEquipments.length === 0 && !searchQuery && (
                  <span className="text-sm text-white/40 italic">絞り込みなし</span>
                )}
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {exercises.map(ex => (
              <div key={ex.id} className="glass bg-[#111] rounded-2xl shadow-sm hover:shadow-[0_0_20px_rgba(255,170,0,0.1)] transition-all border border-white/10 hover:border-primary/50 overflow-hidden group">
                <div className="p-6">
                  <div className="flex justify-between items-start mb-4">
                    <h3 className="text-xl font-bold text-white leading-tight group-hover:text-primary transition-colors">
                      {ex.name}
                    </h3>
                    {ex.level && (
                      <span className="px-3 py-1 bg-white/5 border border-white/10 text-white/80 text-xs font-bold rounded-full whitespace-nowrap ml-2">
                        {displayExerciseLabel(ex.level)}
                      </span>
                    )}
                  </div>
                  
                  <div className="space-y-3 mb-6">
                    {ex.primaryMuscles && ex.primaryMuscles.length > 0 && (
                      <div className="flex items-center text-sm text-white/60">
                        <Activity className="w-4 h-4 mr-2 text-primary" />
                        <span className="font-medium mr-2">メイン:</span>
                        {ex.primaryMuscles.join(', ')}
                      </div>
                    )}
                    {ex.equipment && (
                      <div className="flex items-center text-sm text-white/60">
                        <Dumbbell className="w-4 h-4 mr-2 text-white/40" />
                        <span className="font-medium mr-2">器具:</span>
                        {ex.equipment}
                      </div>
                    )}
                    {ex.category && (
                      <div className="flex items-center text-sm text-white/60">
                        <Shield className="w-4 h-4 mr-2 text-accent" />
                        <span className="font-medium mr-2">分類:</span>
                        {displayExerciseLabel(ex.category)}
                      </div>
                    )}
                  </div>

                  <button 
                    onClick={() => setSelectedExercise(ex)}
                    className="w-full py-3 bg-white/5 hover:bg-primary/20 text-white font-bold rounded-xl transition-colors border border-white/10 hover:border-primary/30 group-hover:text-primary">
                    詳細を見る
                  </button>
                </div>
              </div>
            ))}
          </div>
            {/* 一覧は1ページずつ取得するため、続きがあれば取得手段を示す */}
            {(search.hasMore || search.loadMoreError) && (
              <div className="mt-8 flex flex-col items-center gap-3">
                {search.loadMoreError && (
                  <p role="alert" className="text-sm text-red-200">{search.loadMoreError}</p>
                )}
                {search.hasMore && (
                  <button
                    onClick={search.loadMore}
                    disabled={search.loadingMore}
                    className="px-8 py-3 bg-white/5 hover:bg-white/10 border border-white/10 text-white font-bold rounded-xl transition-colors disabled:opacity-50 flex items-center gap-2"
                  >
                    {search.loadingMore && <Loader2 className="w-5 h-5 animate-spin" />}
                    {search.loadMoreError ? '続きを再試行' : 'さらに表示'}
                  </button>
                )}
              </div>
            )}
          </>
        )}
      </div>

      {/* 種目詳細モーダル */}
      {selectedExercise && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/80 backdrop-blur-md transition-opacity" onClick={() => setSelectedExercise(null)}>
          <div className="bg-[#111] border border-white/10 rounded-3xl shadow-[0_0_50px_rgba(0,0,0,0.5)] max-w-2xl w-full max-h-[85vh] overflow-y-auto animate-in fade-in zoom-in-95 duration-200" onClick={e => e.stopPropagation()}>
            <div className="sticky top-0 bg-[#111]/90 backdrop-blur-xl p-6 border-b border-white/10 flex justify-between items-center z-10">
              <h2 className="text-2xl font-bold text-white pr-4">{selectedExercise.name}</h2>
              <button onClick={() => setSelectedExercise(null)} className="text-white/50 hover:text-white focus:outline-none bg-white/5 hover:bg-white/10 rounded-full w-8 h-8 flex items-center justify-center transition-colors flex-shrink-0">
                <span className="text-xl leading-none">&times;</span>
              </button>
            </div>
            
            <div className="p-6">
              <div className="flex flex-wrap gap-2 mb-6">
                {selectedExercise.level && <span className="px-3 py-1 bg-white/5 border border-white/10 text-white/80 text-xs font-bold rounded-full">{displayExerciseLabel(selectedExercise.level)}</span>}
                {selectedExercise.category && <span className="px-3 py-1 bg-white/5 border border-white/10 text-white/80 text-xs font-bold rounded-full">{displayExerciseLabel(selectedExercise.category)}</span>}
                {detail.exercise?.mechanic && <span className="px-3 py-1 bg-white/5 border border-white/10 text-white/80 text-xs font-bold rounded-full">{displayExerciseLabel(detail.exercise.mechanic)}</span>}
                {detail.exercise?.force && <span className="px-3 py-1 bg-white/5 border border-white/10 text-white/80 text-xs font-bold rounded-full">{displayExerciseLabel(detail.exercise.force)}</span>}
              </div>
              
              <h3 className="text-lg font-bold text-white mb-3 flex items-center">
                <Activity className="w-5 h-5 mr-2 text-primary" />
                鍛えられる筋肉
              </h3>
              <div className="mb-8 bg-black/40 p-4 rounded-2xl border border-white/10">
                <div className="mb-2"><span className="font-semibold text-white/60">メイン: </span><span className="text-white">{selectedExercise.primaryMuscles?.join(', ') || 'なし'}</span></div>
                <div><span className="font-semibold text-white/60">サブ: </span><span className="text-white">{detail.exercise ? detail.exercise.secondaryMuscles?.join(', ') || 'なし' : detail.error ? '-' : '読み込み中...'}</span></div>
              </div>

              <h3 className="text-lg font-bold text-white mb-4 flex items-center">
                <Shield className="w-5 h-5 mr-2 text-accent" />
                トレーニング手順
              </h3>
              <div className="space-y-5">
                {detail.error ? (
                  <div role="alert" className="bg-red-500/10 p-4 rounded-xl text-center border border-red-500/30">
                    <p className="text-red-200 mb-3">{detail.error}</p>
                    <button onClick={detail.retry} className="px-5 py-2 bg-white/10 hover:bg-white/20 text-white text-sm font-bold rounded-xl transition-colors">
                      手順を再取得
                    </button>
                  </div>
                ) : !detail.exercise ? (
                  <p role="status" className="text-white/40 bg-black/40 p-4 rounded-xl text-center border border-white/5 flex items-center justify-center gap-2">
                    <Loader2 className="w-4 h-4 animate-spin" /> 手順を読み込み中...
                  </p>
                ) : detail.exercise.instructions && detail.exercise.instructions.length > 0 ? (
                  detail.exercise.instructions.map((inst, idx) => (
                    <div key={idx} className="flex">
                      <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary/20 text-primary border border-primary/30 flex items-center justify-center font-bold mr-4 mt-0.5 shadow-sm">
                        {idx + 1}
                      </div>
                      <p className="text-white/80 leading-relaxed pt-1 text-justify">{inst}</p>
                    </div>
                  ))
                ) : (
                  <p className="text-white/40 italic bg-black/40 p-4 rounded-xl text-center border border-white/5">手順の詳細はありません。</p>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 種目追加モーダル */}
      {showAddModal && (
        <div className="fixed inset-0 z-[110] flex items-center justify-center p-4 bg-black/80 backdrop-blur-md" onClick={closeAddModal}>
          <div className="bg-[#111] border border-white/10 rounded-3xl shadow-[0_0_50px_rgba(0,0,0,0.5)] max-w-2xl w-full max-h-[85vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <div className="sticky top-0 bg-[#111]/90 backdrop-blur-xl p-6 border-b border-white/10 flex justify-between items-center z-10">
              <div>
                <h2 className="text-2xl font-bold text-white">種目を追加</h2>
                <p className="text-sm text-white/45 mt-1">辞書にない種目を保存して、ワークアウト記録で使えるようにします。</p>
              </div>
              <button onClick={closeAddModal} className="text-white/50 hover:text-white bg-white/5 hover:bg-white/10 rounded-full w-9 h-9 flex items-center justify-center transition-colors flex-shrink-0">
                <span className="text-xl leading-none">&times;</span>
              </button>
            </div>

            <div className="p-6 space-y-5">
              <div className="space-y-2">
                <label className="text-xs font-black text-white/40">種目名 <span className="text-red-400">*</span></label>
                <input
                  type="text"
                  value={newExercise.name}
                  onChange={(e) => setNewExercise({ ...newExercise, name: e.target.value })}
                  placeholder="例: インクラインダンベルプレス"
                  className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
                <div className="space-y-2">
                  <label className="text-xs font-black text-white/40">メイン部位</label>
                  <select
                    value={newExercise.primaryMuscle}
                    onChange={(e) => setNewExercise({ ...newExercise, primaryMuscle: e.target.value })}
                    className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                  >
                    {withCurrent(muscleOptions, newExercise.primaryMuscle).map(m => <option key={m} value={m}>{m}</option>)}
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-xs font-black text-white/40">器具</label>
                  <select
                    value={newExercise.equipment}
                    onChange={(e) => setNewExercise({ ...newExercise, equipment: e.target.value })}
                    className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                  >
                    {withCurrent(filterOptions.equipment, newExercise.equipment).map(eq => <option key={eq} value={eq}>{eq}</option>)}
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-xs font-black text-white/40">分類</label>
                  <select
                    value={newExercise.category}
                    onChange={(e) => setNewExercise({ ...newExercise, category: e.target.value })}
                    className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                  >
                    {CATEGORY_OPTIONS.map(opt => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-xs font-black text-white/40">難易度</label>
                  <select
                    value={newExercise.level}
                    onChange={(e) => setNewExercise({ ...newExercise, level: e.target.value })}
                    className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                  >
                    {DIFFICULTY_OPTIONS.filter(option => option.value !== 'すべて').map(option => (
                      <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="space-y-2">
                <label className="text-xs font-black text-white/40">サブ部位（任意・カンマ区切り）</label>
                <input
                  type="text"
                  value={newExercise.secondaryMuscles}
                  onChange={(e) => setNewExercise({ ...newExercise, secondaryMuscles: e.target.value })}
                  placeholder="例: 肩, 上腕三頭筋"
                  className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                />
              </div>

              <div className="space-y-2">
                <label className="text-xs font-black text-white/40">手順メモ（任意・1行ずつ）</label>
                <textarea
                  value={newExercise.instructions}
                  onChange={(e) => setNewExercise({ ...newExercise, instructions: e.target.value })}
                  placeholder={"例:\nベンチを30度くらいにする\n胸を張って肩をすくめない\n最後までフォーム優先"}
                  className="w-full min-h-[130px] bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                />
              </div>

              {addError && (
                <div className="rounded-2xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200">
                  {addError}
                </div>
              )}

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
                <button
                  onClick={closeAddModal}
                  className="py-4 bg-white/10 text-white font-bold rounded-2xl hover:bg-white/15 transition-all"
                >
                  キャンセル
                </button>
                <button
                  onClick={addExercise}
                  disabled={isAdding || !newExercise.name.trim()}
                  className="py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-40 disabled:pointer-events-none flex items-center justify-center gap-2"
                >
                  {isAdding ? <Loader2 className="w-5 h-5 animate-spin" /> : <Plus className="w-5 h-5" />}
                  辞書に追加
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

    </div>
  )
}
