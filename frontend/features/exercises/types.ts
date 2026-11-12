export interface Exercise {
  id: string;
  name: string;
  force: string;
  level: string;
  mechanic: string;
  equipment: string;
  category: string;
  instructions: string[];
  primaryMuscles: string[];
  secondaryMuscles: string[];
  images: string[];
}

// 一覧APIが返す軽い項目。手順文や画像パスは詳細APIで取得する。
export interface ExerciseSummary {
  id: string;
  name: string;
  level: string | null;
  equipment: string | null;
  category: string | null;
  primaryMuscles: string[];
}

export interface ExercisePage {
  items: ExerciseSummary[];
  total: number;
  limit: number;
  offset: number;
}

export interface MuscleCategory {
  name: string;
  muscles: string[];
}

// 器具・部位の選択肢。種目カタログと同じ表記でバックエンドが定義する。
export interface ExerciseFilterOptions {
  equipment: string[];
  muscle_categories: MuscleCategory[];
}
