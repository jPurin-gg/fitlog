package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/requestctx"
)

func TestLogFeatureOutcomeRecordsCorrelatedValidatedResult(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	ctx := requestctx.WithRequestID(context.Background(), "request-123")

	LogFeatureOutcome(ctx, logger, TaskRecommendation, "invalid_output", time.Now())

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if record["stage"] != "feature" || record["request_id"] != "request-123" || record["task"] != string(TaskRecommendation) || record["outcome"] != "invalid_output" {
		t.Fatalf("feature log = %#v", record)
	}
	if _, ok := record["total_duration_ms"]; !ok {
		t.Fatal("total_duration_ms is missing")
	}
}

func TestLogFeatureOutcomeAllowsDisabledLogger(t *testing.T) {
	LogFeatureOutcome(context.Background(), nil, TaskRecommendation, "applied", time.Now())
}
