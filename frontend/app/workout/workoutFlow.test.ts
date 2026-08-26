import { describe, expect, it } from "vitest";

import { initialRecordFlowState, recordFlowReducer } from "./workoutFlow";

describe("recordFlowReducer", () => {
  it("marks the set as saved before starting the optional recommendation", () => {
    const saving = recordFlowReducer(initialRecordFlowState, { type: "SAVE_STARTED" });
    const saved = recordFlowReducer(saving, { type: "SAVE_SUCCEEDED" });

    expect(saved).toEqual({ save: "saved", recommendation: "idle" });
    expect(recordFlowReducer(saved, { type: "RECOMMENDATION_STARTED" })).toEqual({
      save: "saved",
      recommendation: "loading",
    });
  });

  it("keeps the saved state when the recommendation fails and can retry it", () => {
    const unavailable = recordFlowReducer(
      { save: "saved", recommendation: "loading" },
      { type: "RECOMMENDATION_FAILED" },
    );

    expect(unavailable).toEqual({ save: "saved", recommendation: "unavailable" });
    expect(recordFlowReducer(unavailable, { type: "RECOMMENDATION_STARTED" })).toEqual({
      save: "saved",
      recommendation: "loading",
    });
  });

  it("preserves retryable input state when saving fails", () => {
    expect(recordFlowReducer(
      { save: "saving", recommendation: "idle" },
      { type: "SAVE_FAILED" },
    )).toEqual({ save: "failed", recommendation: "idle" });
  });
});
