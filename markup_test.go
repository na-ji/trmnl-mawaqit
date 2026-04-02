package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"testing"
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
