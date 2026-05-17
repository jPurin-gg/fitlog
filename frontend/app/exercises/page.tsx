"use client"

import React, { useState, useEffect } from 'react'
import { Search, Filter, Dumbbell, Activity, Shield, Plus, Loader2 } from 'lucide-react'

// 今回のマスターデータ（tmpkin_jp.jsonベース）の構造
interface Exercise {
  id: string
  name: string
  force: string
  level: string
  mechanic: string
  equipment: string
  category: string
  instructions: string[]
  primaryMuscles: string[]
  secondaryMuscles: string[]
  images: string[]
}

const MUSCLE_GROUPS = [
  "すべて", "大胸筋", "背中中部", "広背筋", "下背部", "肩", "上腕二頭筋", "上腕三頭筋", 
  "前腕", "腹筋", "大臀筋", "大腿四頭筋", "ハムストリングス", "ふくらはぎ", "首", "僧帽筋"
]

const EQUIPMENTS = [
  "自重", "ダンベル", "バーベル", "マシン", "ケーブル", "バンド", "ケトルベル", "メディシンボール", "バランスボール", "その他"
]

const CATEGORY_OPTIONS = [
  { value: "strength", label: "筋力トレーニング" },
  { value: "cardio", label: "有酸素" },
  { value: "stretching", label: "ストレッチ" },
  { value: "plyometrics", label: "瞬発系" },
  { value: "その他", label: "その他" },
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

function displayExerciseLabel(value?: string) {
  if (!value) return ""
  return EXERCISE_LABELS[value] || value
}

export default function ExercisesPage() {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const [exercises, setExercises] = useState<Exercise[]>([])
  const [muscle, setMuscle] = useState('すべて')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedEquipments, setSelectedEquipments] = useState<string[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [selectedExercise, setSelectedExercise] = useState<Exercise | null>(null)
  const [showAddModal, setShowAddModal] = useState(false)
  const [isAdding, setIsAdding] = useState(false)
  const [addError, setAddError] = useState('')
  const [reloadToken, setReloadToken] = useState(0)
  const [newExercise, setNewExercise] = useState({
    name: '',
    primaryMuscle: '大胸筋',
    secondaryMuscles: '',
    equipment: 'その他',
    category: 'strength',
    instructions: '',
  })

  const toggleEquipment = (eq: string) => {
    if (selectedEquipments.includes(eq)) {
      setSelectedEquipments(selectedEquipments.filter(e => e !== eq))
    } else {
      setSelectedEquipments([...selectedEquipments, eq])
    }
  }

  useEffect(() => {
    const fetchExercises = async () => {
      setIsLoading(true)
      try {
        const params = new URLSearchParams()
        if (muscle !== 'すべて') params.set('muscle', muscle)
        if (searchQuery.trim()) params.set('name', searchQuery.trim())
        if (selectedEquipments.length > 0) params.set('equipment', selectedEquipments.join(','))
        const qs = params.toString()
        const res = await fetch(`${apiUrl}/api/exercises${qs ? '?' + qs : ''}`)
        if (res.ok) {
          const data = await res.json()
          setExercises(data || [])
        }
      } catch (e) {
        console.error("Failed to fetch exercises", e)
      }
      setIsLoading(false)
    }
    // 入力中に毎回叩かないよう 300ms debounce
    const timer = setTimeout(fetchExercises, 300)
    return () => clearTimeout(timer)
  }, [apiUrl, muscle, searchQuery, selectedEquipments, reloadToken])

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
      const res = await fetch(`${apiUrl}/api/exercises/custom`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newExercise.name.trim(),
          category: newExercise.category,
          equipment: newExercise.equipment,
          primary_muscles: newExercise.primaryMuscle ? [newExercise.primaryMuscle] : [],
          secondary_muscles: secondaryMuscles,
          instructions,
        })
      })
      if (!res.ok) {
        setAddError(await res.text() || '種目の追加に失敗しました。')
        return
      }
      const created = await res.json()
      setExercises(prev => [created, ...prev])
      setSelectedExercise(created)
      setReloadToken(prev => prev + 1)
      closeAddModal()
    } catch (e) {
      console.error(e)
      setAddError('通信エラーが発生しました。')
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
            {MUSCLE_GROUPS.map(m => (
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
            <div className="flex items-center text-white/40 text-xs font-bold whitespace-nowrap mr-2"><Dumbbell className="w-4 h-4 mr-1"/> 器具:</div>
            {EQUIPMENTS.map(eq => (
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
        </div>

        {/* 種目リスト */}
        {isLoading ? (
          <div className="flex justify-center items-center py-20">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
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
                <span className="text-primary font-bold text-lg mr-1">{exercises.length}</span> 件の種目
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
                
                {searchQuery && (
                  <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-white/10 text-white/80 border border-white/20">
                    キーワード: &quot;{searchQuery}&quot;
                    <button onClick={() => setSearchQuery('')} className="ml-1.5 text-white/60 hover:text-white font-bold text-sm focus:outline-none leading-none">
                      &times;
                    </button>
                  </span>
                )}

                {muscle === 'すべて' && selectedEquipments.length === 0 && !searchQuery && (
                  <span className="text-sm text-white/40 italic">絞り込みなし（全件表示）</span>
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
                {selectedExercise.mechanic && <span className="px-3 py-1 bg-white/5 border border-white/10 text-white/80 text-xs font-bold rounded-full">{displayExerciseLabel(selectedExercise.mechanic)}</span>}
                {selectedExercise.force && <span className="px-3 py-1 bg-white/5 border border-white/10 text-white/80 text-xs font-bold rounded-full">{displayExerciseLabel(selectedExercise.force)}</span>}
              </div>
              
              <h3 className="text-lg font-bold text-white mb-3 flex items-center">
                <Activity className="w-5 h-5 mr-2 text-primary" />
                鍛えられる筋肉
              </h3>
              <div className="mb-8 bg-black/40 p-4 rounded-2xl border border-white/10">
                <div className="mb-2"><span className="font-semibold text-white/60">メイン: </span><span className="text-white">{selectedExercise.primaryMuscles?.join(', ') || 'なし'}</span></div>
                <div><span className="font-semibold text-white/60">サブ: </span><span className="text-white">{selectedExercise.secondaryMuscles?.join(', ') || 'なし'}</span></div>
              </div>

              <h3 className="text-lg font-bold text-white mb-4 flex items-center">
                <Shield className="w-5 h-5 mr-2 text-accent" />
                トレーニング手順
              </h3>
              <div className="space-y-5">
                {selectedExercise.instructions && selectedExercise.instructions.length > 0 ? (
                  selectedExercise.instructions.map((inst, idx) => (
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

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div className="space-y-2">
                  <label className="text-xs font-black text-white/40">メイン部位</label>
                  <select
                    value={newExercise.primaryMuscle}
                    onChange={(e) => setNewExercise({ ...newExercise, primaryMuscle: e.target.value })}
                    className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                  >
                    {MUSCLE_GROUPS.filter(m => m !== 'すべて').map(m => <option key={m} value={m}>{m}</option>)}
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-xs font-black text-white/40">器具</label>
                  <select
                    value={newExercise.equipment}
                    onChange={(e) => setNewExercise({ ...newExercise, equipment: e.target.value })}
                    className="w-full bg-black/40 border border-white/20 rounded-xl px-4 py-3 focus:outline-none focus:border-primary text-white"
                  >
                    {EQUIPMENTS.map(eq => <option key={eq} value={eq}>{eq}</option>)}
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
