export interface DayRoutine {
  day_name: string;
  target: string;
  example_exercises: string[];
  exercise_ids?: string[];
}

export interface MonthlyPlan {
  id?: number;
  plan_month?: string;
  plan_name: string;
  frequency: string;
  description: string;
  rationale: string;
  rest_days?: number[];
  recommended_days: number[];
  weekly_routine: DayRoutine[];
}

export interface WorkedOutDay {
  date: number;
  workout_id: number;
  type: string;
}

export interface PlannedWorkoutDay {
  date: number;
  plan_id: number;
  target: string;
}

export interface CalendarWorkoutSet {
  id?: number;
  exercise_id: string;
  exercise_name?: string;
  weight: number;
  reps: number;
  set_order: number;
  feeling: string;
}

export interface CalendarWorkout {
  workout_id: number;
  date: string;
  title: string;
  sets: CalendarWorkoutSet[];
}

export interface CalendarPlanExercise {
  exercise_id: string;
  name: string;
  planned_sets: number;
  target_weight: number;
  target_reps: number;
  last_max_weight?: number;
}

export interface CalendarPlan {
  id?: number;
  workout_id?: number;
  plan_date: string;
  status: string;
  plan: {
    workout_title: string;
    target: string;
    estimated_duration_min: number;
    coach_note: string;
    exercises: CalendarPlanExercise[];
  };
}
