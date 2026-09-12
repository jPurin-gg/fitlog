export interface WorkoutPlanExercise {
  exercise_id: string;
  name: string;
  planned_sets: number;
  target_weight: number;
  target_reps: number;
  last_max_weight?: number;
}

export interface WorkoutPlanSession {
  id: number;
  workout_id: number;
  plan_date: string;
  status: string;
  ai_status?: "applied" | "fallback" | "not_requested";
  plan: {
    workout_title: string;
    target: string;
    estimated_duration_min: number;
    coach_note: string;
    exercises: WorkoutPlanExercise[];
  };
}

export interface SetRecommendation {
  next_action: "CONTINUE" | "STOP" | "ADJUST";
  recommendation: string;
  target_weight: number;
  target_reps: number;
  reason: string;
  record_template?: string;
  max_weight: number;
}

export interface WorkoutSummaryExercise {
  exercise_id: string;
  name: string;
  sets: number;
  total_reps: number;
  best_weight: number;
  total_volume: number;
}

export interface WorkoutSummary {
  total_sets: number;
  total_reps: number;
  total_volume: number;
  duration_min: number;
  pr_count: number;
  ai_comment?: string;
  exercises: WorkoutSummaryExercise[];
}

export interface WorkoutDetail {
  id: number;
  title: string;
  started_at: string;
  ended_at: string;
  status: string;
  summary: WorkoutSummary;
}

export interface SetInput {
  exercise_id: string;
  set_order: number;
  weight: number;
  reps: number;
  feeling: string;
}
