export type SaveState = "editing" | "saving" | "saved" | "failed";
export type RecommendationState = "idle" | "loading" | "ready" | "unavailable";

export interface RecordFlowState {
  save: SaveState;
  recommendation: RecommendationState;
}

export type RecordFlowAction =
  | { type: "RESET" }
  | { type: "SAVE_STARTED" }
  | { type: "SAVE_SUCCEEDED" }
  | { type: "SAVE_FAILED" }
  | { type: "RECOMMENDATION_STARTED" }
  | { type: "RECOMMENDATION_SUCCEEDED" }
  | { type: "RECOMMENDATION_FAILED" };

export const initialRecordFlowState: RecordFlowState = {
  save: "editing",
  recommendation: "idle",
};

export function recordFlowReducer(state: RecordFlowState, action: RecordFlowAction): RecordFlowState {
  switch (action.type) {
    case "RESET":
      return initialRecordFlowState;
    case "SAVE_STARTED":
      return { save: "saving", recommendation: "idle" };
    case "SAVE_SUCCEEDED":
      return { save: "saved", recommendation: "idle" };
    case "SAVE_FAILED":
      return { save: "failed", recommendation: "idle" };
    case "RECOMMENDATION_STARTED":
      return { ...state, recommendation: "loading" };
    case "RECOMMENDATION_SUCCEEDED":
      return { ...state, recommendation: "ready" };
    case "RECOMMENDATION_FAILED":
      return { ...state, recommendation: "unavailable" };
  }
}
