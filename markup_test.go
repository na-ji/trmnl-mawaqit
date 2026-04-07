package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"testing"
	"time"
)

func TestTimeToMinutes(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"05:30", 330, false},
		{"12:00", 720, false},
		{"23:59", 1439, false},
		{"00:00", 0, false},
		{" 5 : 30 ", 330, false},
		{"invalid", 0, true},
		{"12:ab", 0, true},
		{"ab:30", 0, true},
	}
	for _, tt := range tests {
		got, err := timeToMinutes(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("timeToMinutes(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("timeToMinutes(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestBuildPrayerDisplay(t *testing.T) {
	data := makeMawaqitResponse(t, "Test Mosque", "13:00", nil, nil,
		// January (month 0), day 1
		[]string{"06:00", "07:30", "12:30", "15:45", "18:00", "19:30"},
	)

	pd, err := buildPrayerDisplay(data, "UTC")
	if err != nil {
		t.Fatalf("buildPrayerDisplay: %v", err)
	}

	if pd.MosqueName != "Test Mosque" {
		t.Errorf("MosqueName = %q, want %q", pd.MosqueName, "Test Mosque")
	}
	if len(pd.Prayers) != 6 {
		t.Fatalf("len(Prayers) = %d, want 6", len(pd.Prayers))
	}

	expectedNames := []string{"Fajr", "Shuruq", "Dohr", "Asr", "Maghrib", "Isha"}
	for i, name := range expectedNames {
		if pd.Prayers[i].Name != name {
			t.Errorf("Prayers[%d].Name = %q, want %q", i, pd.Prayers[i].Name, name)
		}
	}

	if pd.Jumua != "13:00" {
		t.Errorf("Jumua = %q, want %q", pd.Jumua, "13:00")
	}
}

func TestBuildPrayerDisplayAfterIsha(t *testing.T) {
	todayTimes := []string{"05:30", "07:00", "12:30", "15:45", "18:00", "19:30"}
	tomorrowTimes := []string{"05:45", "07:15", "12:35", "15:50", "18:05", "19:35"}

	data := makeMawaqitResponseDays(t, "Test Mosque", "13:00", nil, nil, map[dayKey][]string{
		{month: 0, day: 15}: todayTimes,
		{month: 0, day: 16}: tomorrowTimes,
	})

	// 20:00 on Jan 15 — after Isha (19:30)
	orig := nowFunc
	nowFunc = func() time.Time {
		return time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC)
	}
	defer func() { nowFunc = orig }()

	pd, err := buildPrayerDisplay(data, "UTC")
	if err != nil {
		t.Fatalf("buildPrayerDisplay: %v", err)
	}

	// Should display tomorrow's times
	for i, want := range tomorrowTimes {
		if pd.Prayers[i].Time != want {
			t.Errorf("Prayers[%d].Time = %q, want %q (tomorrow)", i, pd.Prayers[i].Time, want)
		}
	}

	// Fajr should be marked as next
	if !pd.Prayers[0].IsNext {
		t.Error("expected Fajr to be marked as next prayer after Isha")
	}
	for i := 1; i < len(pd.Prayers); i++ {
		if pd.Prayers[i].IsNext {
			t.Errorf("Prayers[%d] (%s) should not be marked as next", i, pd.Prayers[i].Name)
		}
	}

	// Cache expiry should be tomorrow's Fajr
	wantExpiry := time.Date(2025, 1, 16, 5, 45, 0, 0, time.UTC)
	if !pd.NextPrayerTime.Equal(wantExpiry) {
		t.Errorf("NextPrayerTime = %v, want %v", pd.NextPrayerTime, wantExpiry)
	}
}

func TestBuildPrayerDisplayBeforeFajr(t *testing.T) {
	todayTimes := []string{"05:30", "07:00", "12:30", "15:45", "18:00", "19:30"}

	data := makeMawaqitResponse(t, "Test Mosque", "13:00", nil, nil, todayTimes)

	// 00:30 on any day — before Fajr, should show today's times
	orig := nowFunc
	nowFunc = func() time.Time {
		return time.Date(2025, 1, 15, 0, 30, 0, 0, time.UTC)
	}
	defer func() { nowFunc = orig }()

	pd, err := buildPrayerDisplay(data, "UTC")
	if err != nil {
		t.Fatalf("buildPrayerDisplay: %v", err)
	}

	// Should display today's times (not tomorrow's)
	for i, want := range todayTimes {
		if pd.Prayers[i].Time != want {
			t.Errorf("Prayers[%d].Time = %q, want %q (today)", i, pd.Prayers[i].Time, want)
		}
	}

	// Fajr should be next
	if !pd.Prayers[0].IsNext {
		t.Error("expected Fajr to be marked as next prayer before Fajr")
	}

	// Cache expiry should be today's Fajr (not tomorrow)
	wantExpiry := time.Date(2025, 1, 15, 5, 30, 0, 0, time.UTC)
	if !pd.NextPrayerTime.Equal(wantExpiry) {
		t.Errorf("NextPrayerTime = %v, want %v", pd.NextPrayerTime, wantExpiry)
	}
}

func TestBuildPrayerDisplayAfterIshaDSTTransition(t *testing.T) {
	// US/Eastern: March 9, 2025 clocks spring forward from 2:00 to 3:00 (EST → EDT)
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone not available: %v", err)
	}

	mar8Times := []string{"05:30", "07:00", "12:30", "15:45", "18:00", "19:30"}
	mar9Times := []string{"05:25", "06:55", "12:28", "15:43", "18:05", "19:35"}

	data := makeMawaqitResponseDays(t, "Test Mosque", "13:00", nil, nil, map[dayKey][]string{
		{month: 2, day: 8}: mar8Times, // March 8 (EST)
		{month: 2, day: 9}: mar9Times, // March 9 (EDT, clocks spring forward)
	})

	// 20:00 EST on March 8 — after Isha, tomorrow has DST change
	orig := nowFunc
	nowFunc = func() time.Time {
		return time.Date(2025, 3, 8, 20, 0, 0, 0, loc)
	}
	defer func() { nowFunc = orig }()

	pd, err := buildPrayerDisplay(data, "America/New_York")
	if err != nil {
		t.Fatalf("buildPrayerDisplay: %v", err)
	}

	// Should display March 9 times
	for i, want := range mar9Times {
		if pd.Prayers[i].Time != want {
			t.Errorf("Prayers[%d].Time = %q, want %q (Mar 9 EDT)", i, pd.Prayers[i].Time, want)
		}
	}

	if !pd.Prayers[0].IsNext {
		t.Error("expected Fajr to be marked as next prayer")
	}

	// Cache expiry should be March 9 05:25 EDT (= 09:25 UTC, since EDT is UTC-4)
	wantExpiry := time.Date(2025, 3, 9, 5, 25, 0, 0, loc)
	if !pd.NextPrayerTime.Equal(wantExpiry) {
		t.Errorf("NextPrayerTime = %v, want %v", pd.NextPrayerTime, wantExpiry)
	}
	// Verify the UTC offset changed (EDT not EST)
	_, offset := pd.NextPrayerTime.In(loc).Zone()
	if offset != -4*3600 {
		t.Errorf("expected EDT (UTC-4) offset, got %d", offset)
	}
}

func TestBuildPrayerDisplayAfterIshaMonthBoundary(t *testing.T) {
	janTimes := []string{"05:30", "07:00", "12:30", "15:45", "18:00", "19:30"}
	febTimes := []string{"05:15", "06:45", "12:25", "15:40", "18:10", "19:40"}

	data := makeMawaqitResponseDays(t, "Test Mosque", "13:00", nil, nil, map[dayKey][]string{
		{month: 0, day: 31}: janTimes, // Jan 31
		{month: 1, day: 1}:  febTimes, // Feb 1
	})

	// 21:00 on Jan 31 — after Isha, tomorrow is Feb 1
	orig := nowFunc
	nowFunc = func() time.Time {
		return time.Date(2025, 1, 31, 21, 0, 0, 0, time.UTC)
	}
	defer func() { nowFunc = orig }()

	pd, err := buildPrayerDisplay(data, "UTC")
	if err != nil {
		t.Fatalf("buildPrayerDisplay: %v", err)
	}

	// Should display Feb 1 times
	for i, want := range febTimes {
		if pd.Prayers[i].Time != want {
			t.Errorf("Prayers[%d].Time = %q, want %q (Feb 1)", i, pd.Prayers[i].Time, want)
		}
	}

	if !pd.Prayers[0].IsNext {
		t.Error("expected Fajr to be marked as next prayer")
	}

	// Cache expiry should be Feb 1 Fajr
	wantExpiry := time.Date(2025, 2, 1, 5, 15, 0, 0, time.UTC)
	if !pd.NextPrayerTime.Equal(wantExpiry) {
		t.Errorf("NextPrayerTime = %v, want %v", pd.NextPrayerTime, wantExpiry)
	}
}

func TestBuildPrayerDisplayMidDay(t *testing.T) {
	todayTimes := []string{"05:30", "07:00", "12:30", "15:45", "18:00", "19:30"}

	data := makeMawaqitResponse(t, "Test Mosque", "13:00", nil, nil, todayTimes)

	// 13:00 — between Dohr (12:30) and Asr (15:45)
	orig := nowFunc
	nowFunc = func() time.Time {
		return time.Date(2025, 1, 15, 13, 0, 0, 0, time.UTC)
	}
	defer func() { nowFunc = orig }()

	pd, err := buildPrayerDisplay(data, "UTC")
	if err != nil {
		t.Fatalf("buildPrayerDisplay: %v", err)
	}

	// Asr (index 3) should be next
	if !pd.Prayers[3].IsNext {
		t.Error("expected Asr to be marked as next prayer")
	}

	// Cache expiry should be today's Asr
	wantExpiry := time.Date(2025, 1, 15, 15, 45, 0, 0, time.UTC)
	if !pd.NextPrayerTime.Equal(wantExpiry) {
		t.Errorf("NextPrayerTime = %v, want %v", pd.NextPrayerTime, wantExpiry)
	}
}

func TestBuildPrayerDisplayWithJumua2And3(t *testing.T) {
	j2 := "13:30"
	j3 := "14:00"
	data := makeMawaqitResponse(t, "Multi Jumua Mosque", "13:00", &j2, &j3,
		[]string{"06:00", "07:30", "12:30", "15:45", "18:00", "19:30"},
	)

	pd, err := buildPrayerDisplay(data, "UTC")
	if err != nil {
		t.Fatalf("buildPrayerDisplay: %v", err)
	}

	if pd.Jumua2 != "13:30" {
		t.Errorf("Jumua2 = %q, want %q", pd.Jumua2, "13:30")
	}
	if pd.Jumua3 != "14:00" {
		t.Errorf("Jumua3 = %q, want %q", pd.Jumua3, "14:00")
	}
}

func TestBuildPrayerDisplayInvalidTimezone(t *testing.T) {
	data := makeMawaqitResponse(t, "Test", "13:00", nil, nil,
		[]string{"06:00", "07:30", "12:30", "15:45", "18:00", "19:30"},
	)

	// Should fall back to UTC without error
	pd, err := buildPrayerDisplay(data, "Invalid/Zone")
	if err != nil {
		t.Fatalf("buildPrayerDisplay with invalid timezone: %v", err)
	}
	if pd == nil {
		t.Fatal("expected non-nil PrayerDisplay")
	}
}

func TestRenderAllMarkup(t *testing.T) {
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	pd := &PrayerDisplay{
		MosqueName: "Test Mosque",
		Prayers: []Prayer{
			{Name: "Fajr", Time: "06:00", IsNext: true},
			{Name: "Shuruq", Time: "07:30", IsNext: false},
			{Name: "Dohr", Time: "12:30", IsNext: false},
			{Name: "Asr", Time: "15:45", IsNext: false},
			{Name: "Maghrib", Time: "18:00", IsNext: false},
			{Name: "Isha", Time: "19:30", IsNext: false},
		},
		Jumua: "13:00",
	}

	result, err := renderAllMarkup(tmpl, pd)
	if err != nil {
		t.Fatalf("renderAllMarkup: %v", err)
	}

	// All four layout variants should be non-empty
	if result.Markup == "" {
		t.Error("Markup (full) is empty")
	}
	if result.MarkupHalfHoriz == "" {
		t.Error("MarkupHalfHoriz is empty")
	}
	if result.MarkupHalfVert == "" {
		t.Error("MarkupHalfVert is empty")
	}
	if result.MarkupQuadrant == "" {
		t.Error("MarkupQuadrant is empty")
	}

	// Check that prayer times and mosque name appear in full markup
	for _, p := range pd.Prayers {
		if !contains(result.Markup, p.Time) {
			t.Errorf("full markup missing prayer time %q", p.Time)
		}
		if !contains(result.Markup, p.Name) {
			t.Errorf("full markup missing prayer name %q", p.Name)
		}
	}
	if !contains(result.Markup, "Test Mosque") {
		t.Error("full markup missing mosque name")
	}
}

func TestGetDayTimes(t *testing.T) {
	data := makeMawaqitResponse(t, "Test", "13:00", nil, nil,
		[]string{"06:00", "07:30", "12:30", "15:45", "18:00", "19:30"},
	)

	times, err := data.GetDayTimes(0, 1)
	if err != nil {
		t.Fatalf("GetDayTimes: %v", err)
	}
	if len(times) != 6 {
		t.Fatalf("len(times) = %d, want 6", len(times))
	}
	if times[0] != "06:00" {
		t.Errorf("times[0] = %q, want %q", times[0], "06:00")
	}

	// Out of range month
	_, err = data.GetDayTimes(12, 1)
	if err == nil {
		t.Error("expected error for out-of-range month")
	}

	// Missing day
	_, err = data.GetDayTimes(0, 99)
	if err == nil {
		t.Error("expected error for missing day")
	}
}

// helpers

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type dayKey struct {
	month int // 0-indexed
	day   int // 1-indexed
}

func makeMawaqitResponseDays(t *testing.T, name, jumua string, jumua2, jumua3 *string, days map[dayKey][]string) *MawaqitResponse {
	t.Helper()
	calendar := make([]map[string]json.RawMessage, 12)
	for i := range calendar {
		calendar[i] = make(map[string]json.RawMessage)
	}
	for dk, times := range days {
		timesJSON, err := json.Marshal(times)
		if err != nil {
			t.Fatalf("marshal times: %v", err)
		}
		calendar[dk.month][fmt.Sprintf("%d", dk.day)] = json.RawMessage(timesJSON)
	}

	data := &MawaqitResponse{}
	data.RawData.Name = name
	data.RawData.Jumua = jumua
	data.RawData.Jumua2 = jumua2
	data.RawData.Jumua3 = jumua3
	data.RawData.Calendar = calendar
	return data
}

func makeMawaqitResponse(t *testing.T, name, jumua string, jumua2, jumua3 *string, dayTimes []string) *MawaqitResponse {
	t.Helper()
	timesJSON, err := json.Marshal(dayTimes)
	if err != nil {
		t.Fatalf("marshal times: %v", err)
	}

	// Build a full 12-month calendar with every day (1-31) having the same times.
	month := make(map[string]json.RawMessage)
	for d := 1; d <= 31; d++ {
		month[fmt.Sprintf("%d", d)] = json.RawMessage(timesJSON)
	}
	calendar := make([]map[string]json.RawMessage, 12)
	for i := range calendar {
		calendar[i] = month
	}

	data := &MawaqitResponse{}
	data.RawData.Name = name
	data.RawData.Jumua = jumua
	data.RawData.Jumua2 = jumua2
	data.RawData.Jumua3 = jumua3
	data.RawData.Calendar = calendar
	return data
}
