//go:build integration

package app_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"reflect"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/auth"
	"github.com/jPurin-gg/myfitlog-backend/internal/body"
	"github.com/jPurin-gg/myfitlog-backend/internal/reporting"
)

const measurementsPath = "/api/body-measurements"

func TestBodyMeasurementsCRUDAndNullsHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	assertMeasurements(t, f, []body.Measurement{})

	// Omitted optional values must round-trip through SQL NULL, not zero.
	created := createMeasurement(t, f, body.MeasurementInput{MeasuredOn: "2026-11-10", WeightKg: 70.25})
	want := body.Measurement{ID: created.ID, MeasuredOn: "2026-11-10", WeightKg: 70.25}
	assertMeasurement(t, created, want)
	assertStoredMeasurement(t, f.db, want)
	assertMeasurements(t, f, []body.Measurement{want})

	path := fmt.Sprintf("%s/%d", measurementsPath, created.ID)
	// A recorded zero body-fat percentage must remain distinct from NULL.
	fat, rate := 0.0, 65
	updated := decodeAPI[body.Measurement](t, f.request(t, "PUT", path, body.MeasurementInput{
		MeasuredOn: "2026-11-11", WeightKg: 69.5, BodyFatPercent: &fat, RestingHeartRate: &rate,
	}, "", http.StatusOK))
	want = body.Measurement{ID: created.ID, MeasuredOn: "2026-11-11", WeightKg: 69.5, BodyFatPercent: &fat, RestingHeartRate: &rate}
	assertMeasurement(t, updated, want)
	assertStoredMeasurement(t, f.db, want)
	assertMeasurements(t, f, []body.Measurement{want})

	cleared := decodeAPI[body.Measurement](t, f.request(t, "PUT", path, map[string]any{
		"measured_on": "2026-11-11", "weight_kg": 69.25,
		"body_fat_percent": nil, "resting_heart_rate": nil,
	}, "", http.StatusOK))
	want = body.Measurement{ID: created.ID, MeasuredOn: "2026-11-11", WeightKg: 69.25}
	assertMeasurement(t, cleared, want)
	assertStoredMeasurement(t, f.db, want)
	assertMeasurements(t, f, []body.Measurement{want})

	if response := f.request(t, "DELETE", path, nil, "", http.StatusNoContent); len(response) != 0 {
		t.Fatalf("DELETE response = %s, want empty body", response)
	}
	assertMeasurements(t, f, []body.Measurement{})
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM body_measurements WHERE id=$1`, created.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("deleted measurement count = %d, want 0", count)
	}
	f.request(t, "PUT", path, body.MeasurementInput{MeasuredOn: want.MeasuredOn, WeightKg: 68}, "", http.StatusNotFound)
	f.request(t, "DELETE", path, nil, "", http.StatusNotFound)
}

func TestBodyMeasurementsRejectDuplicateDatesHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	first := createMeasurement(t, f, body.MeasurementInput{MeasuredOn: "2026-11-10", WeightKg: 70})
	second := createMeasurement(t, f, body.MeasurementInput{MeasuredOn: "2026-11-11", WeightKg: 69.5})
	duplicate := body.MeasurementInput{MeasuredOn: first.MeasuredOn, WeightKg: 80}
	f.request(t, "POST", measurementsPath, duplicate, "", http.StatusConflict)
	f.request(t, "PUT", fmt.Sprintf("%s/%d", measurementsPath, second.ID), duplicate, "", http.StatusConflict)
	assertMeasurements(t, f, []body.Measurement{second, first})
	assertStoredMeasurement(t, f.db, first)
	assertStoredMeasurement(t, f.db, second)

	// Keeping a record's own date is a valid update, not a duplicate.
	first.WeightKg = 69
	updated := decodeAPI[body.Measurement](t, f.request(t, "PUT", fmt.Sprintf("%s/%d", measurementsPath, first.ID),
		body.MeasurementInput{MeasuredOn: first.MeasuredOn, WeightKg: first.WeightKg}, "", http.StatusOK))
	assertMeasurement(t, updated, first)
	assertMeasurements(t, f, []body.Measurement{second, first})
}

func TestBodyMeasurementsAreIsolatedBetweenUsersHTTP(t *testing.T) {
	owner := newManualAPIFixture(t, nil)
	owned := createMeasurement(t, owner, body.MeasurementInput{MeasuredOn: "2026-11-10", WeightKg: 70})
	other := anotherAPIUser(t, owner)
	assertMeasurements(t, other, []body.Measurement{})
	assertBodyStats(t, other)

	// The unique date constraint is per user, not global.
	otherRecord := createMeasurement(t, other, body.MeasurementInput{MeasuredOn: owned.MeasuredOn, WeightKg: 90})
	path := fmt.Sprintf("%s/%d", measurementsPath, owned.ID)
	other.request(t, "PUT", path, body.MeasurementInput{MeasuredOn: "2026-11-12", WeightKg: 99}, "", http.StatusNotFound)
	other.request(t, "DELETE", path, nil, "", http.StatusNotFound)
	assertMeasurements(t, owner, []body.Measurement{owned})
	assertMeasurements(t, other, []body.Measurement{otherRecord})
	assertStoredMeasurement(t, owner.db, owned)
	assertBodyStats(t, owner, reporting.Stat{Label: "体重", Value: "70", Unit: "kg"})
	assertBodyStats(t, other, reporting.Stat{Label: "体重", Value: "90", Unit: "kg"})
}

func TestBodyMeasurementsUpdateDashboardStatisticsHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	other := anotherAPIUser(t, f)
	otherRate := 110
	createMeasurement(t, other, body.MeasurementInput{MeasuredOn: "2026-11-12", WeightKg: 130, RestingHeartRate: &otherRate})
	assertBodyStats(t, f)

	// Insert out of date order. Heart rate uses the newest non-NULL values,
	// independently of the newest two weights.
	oldRate, middleRate := 75, 72
	oldest := createMeasurement(t, f, body.MeasurementInput{MeasuredOn: "2026-11-07", WeightKg: 72.5, RestingHeartRate: &oldRate})
	newest := createMeasurement(t, f, body.MeasurementInput{MeasuredOn: "2026-11-11", WeightKg: 70})
	middle := createMeasurement(t, f, body.MeasurementInput{MeasuredOn: "2026-11-09", WeightKg: 71.5, RestingHeartRate: &middleRate})
	assertMeasurements(t, f, []body.Measurement{newest, middle, oldest})
	assertBodyStats(t, f,
		reporting.Stat{Label: "体重", Value: "70", Unit: "kg", Trend: "-1.5kg"},
		reporting.Stat{Label: "心拍数", Value: "72", Unit: "bpm", Trend: "-3bpm"},
	)

	newestPath := fmt.Sprintf("%s/%d", measurementsPath, newest.ID)
	newRate := 68
	input := body.MeasurementInput{MeasuredOn: newest.MeasuredOn, WeightKg: 69.5, RestingHeartRate: &newRate}
	f.request(t, "PUT", newestPath, input, "", http.StatusOK)
	assertBodyStats(t, f,
		reporting.Stat{Label: "体重", Value: "69.5", Unit: "kg", Trend: "-2.0kg"},
		reporting.Stat{Label: "心拍数", Value: "68", Unit: "bpm", Trend: "-4bpm"},
	)

	// Omitting a previously recorded optional value also clears it to NULL.
	input.RestingHeartRate = nil
	f.request(t, "PUT", newestPath, input, "", http.StatusOK)
	assertBodyStats(t, f,
		reporting.Stat{Label: "体重", Value: "69.5", Unit: "kg", Trend: "-2.0kg"},
		reporting.Stat{Label: "心拍数", Value: "72", Unit: "bpm", Trend: "-3bpm"},
	)
	f.request(t, "DELETE", fmt.Sprintf("%s/%d", measurementsPath, middle.ID), nil, "", http.StatusNoContent)
	assertBodyStats(t, f,
		reporting.Stat{Label: "体重", Value: "69.5", Unit: "kg", Trend: "-3.0kg"},
		reporting.Stat{Label: "心拍数", Value: "75", Unit: "bpm"},
	)
	f.request(t, "DELETE", fmt.Sprintf("%s/%d", measurementsPath, oldest.ID), nil, "", http.StatusNoContent)
	assertBodyStats(t, f, reporting.Stat{Label: "体重", Value: "69.5", Unit: "kg"})
	f.request(t, "DELETE", newestPath, nil, "", http.StatusNoContent)
	assertBodyStats(t, f)
	if calls := f.aiCalls.Load(); calls != 0 {
		t.Fatalf("AI calls = %d, want 0", calls)
	}
}

func TestDashboardWorkoutStatisticsHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	other := anotherAPIUser(t, f)
	ownerUser := decodeAPI[auth.User](t, f.request(t, "GET", "/api/auth/me", nil, "", http.StatusOK))
	otherUser := decodeAPI[auth.User](t, other.request(t, "GET", "/api/auth/me", nil, "", http.StatusOK))
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start, end := today.AddDate(0, 0, -6), today.AddDate(0, 0, 1)
	previousStart := start.AddDate(0, 0, -7)
	for _, record := range []struct {
		userID  int
		started time.Time
		minutes int
	}{
		{ownerUser.ID, start, 30},
		{ownerUser.ID, today.Add(12 * time.Hour), 15},
		{ownerUser.ID, previousStart, 20},
		{ownerUser.ID, previousStart.Add(-time.Hour), 300},
		{ownerUser.ID, end, 300},
		{otherUser.ID, start, 300},
	} {
		if _, err := f.db.Exec(`INSERT INTO workouts (user_id,started_at,ended_at) VALUES ($1,$2,$3)`,
			record.userID, record.started, record.started.Add(time.Duration(record.minutes)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	// An unfinished workout today must not contribute to duration statistics.
	if _, err := f.db.Exec(`INSERT INTO workouts (user_id,started_at) VALUES ($1,$2)`, ownerUser.ID, today); err != nil {
		t.Fatal(err)
	}
	dashboard := decodeAPI[reporting.Dashboard](t, f.request(t, "GET", "/api/dashboard", nil, "", http.StatusOK))
	want := []reporting.Stat{
		{Label: "消費カロリー", Value: "225", Unit: "kcal", Trend: "+125%"},
		{Label: "活動時間", Value: "45", Unit: "分", Trend: "+25分"},
	}
	if !reflect.DeepEqual(dashboard.Stats, want) {
		t.Fatalf("workout stats = %+v, want %+v", dashboard.Stats, want)
	}
}

func anotherAPIUser(t *testing.T, f *manualAPIFixture) *manualAPIFixture {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	other := &manualAPIFixture{db: f.db, server: f.server, client: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
	other.request(t, "POST", "/api/auth/login", map[string]string{"nickname": "other-integration-user", "password": "test-only-password"}, "", http.StatusCreated)
	other.request(t, "GET", "/api/auth/me", nil, "", http.StatusOK)
	return other
}

func createMeasurement(t *testing.T, f *manualAPIFixture, input body.MeasurementInput) body.Measurement {
	t.Helper()
	measurement := decodeAPI[body.Measurement](t, f.request(t, "POST", measurementsPath, input, "", http.StatusCreated))
	if measurement.ID <= 0 {
		t.Fatalf("created measurement ID = %d, want positive", measurement.ID)
	}
	assertMeasurement(t, measurement, body.Measurement{
		ID: measurement.ID, MeasuredOn: input.MeasuredOn, WeightKg: input.WeightKg,
		BodyFatPercent: input.BodyFatPercent, RestingHeartRate: input.RestingHeartRate,
	})
	return measurement
}

func assertMeasurement(t *testing.T, got, want body.Measurement) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("measurement = %+v, want %+v", got, want)
	}
}

func assertMeasurements(t *testing.T, f *manualAPIFixture, want []body.Measurement) {
	t.Helper()
	got := decodeAPI[[]body.Measurement](t, f.request(t, "GET", measurementsPath, nil, "", http.StatusOK))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("measurements = %+v, want %+v", got, want)
	}
}

func assertStoredMeasurement(t *testing.T, db *sql.DB, want body.Measurement) {
	t.Helper()
	var got body.Measurement
	var fat sql.NullFloat64
	var rate sql.NullInt64
	if err := db.QueryRow(`SELECT id,measured_on::text,weight_kg,body_fat_percent,resting_heart_rate
		FROM body_measurements WHERE id=$1`, want.ID).Scan(&got.ID, &got.MeasuredOn, &got.WeightKg, &fat, &rate); err != nil {
		t.Fatal(err)
	}
	if fat.Valid {
		got.BodyFatPercent = &fat.Float64
	}
	if rate.Valid {
		value := int(rate.Int64)
		got.RestingHeartRate = &value
	}
	assertMeasurement(t, got, want)
}

func assertBodyStats(t *testing.T, f *manualAPIFixture, want ...reporting.Stat) {
	t.Helper()
	dashboard := decodeAPI[reporting.Dashboard](t, f.request(t, "GET", "/api/dashboard", nil, "", http.StatusOK))
	var got []reporting.Stat
	for _, stat := range dashboard.Stats {
		if stat.Label == "体重" || stat.Label == "心拍数" {
			got = append(got, stat)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body stats = %+v, want %+v", got, want)
	}
}
