package main

import (
	"bytes"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"
)

type Prayer struct {
	Name   string
	Time   string
	IsNext bool
}

type PrayerDisplay struct {
	MosqueName string
	Prayers    []Prayer
	Jumua      string
	Jumua2     string
	Jumua3     string
}

var names = []string{"Fajr", "Shuruq", "Dohr", "Asr", "Maghrib", "Isha"}

func buildPrayerDisplay(data *MawaqitResponse, timezone string) (*PrayerDisplay, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	month := int(now.Month()) - 1 // 0-indexed
	day := now.Day()              // 1-indexed

	times, err := data.GetDayTimes(month, day)
	if err != nil {
		return nil, err
	}

	// Determine next prayer
	nowMinutes := now.Hour()*60 + now.Minute()
	nextIdx := 0 // default to Fajr (wrap around)
	for i, t := range times {
		m, err := timeToMinutes(t)
		if err != nil {
			continue
		}
		if m > nowMinutes {
			nextIdx = i
			break
		}
	}

	prayers := make([]Prayer, len(names))
	for i, name := range names {
		prayers[i] = Prayer{
			Name:   name,
			Time:   times[i],
			IsNext: i == nextIdx,
		}
	}

	pd := &PrayerDisplay{
		MosqueName: data.RawData.Name,
		Prayers:    prayers,
		Jumua:      data.RawData.Jumua,
	}
	if data.RawData.Jumua2 != nil {
		pd.Jumua2 = *data.RawData.Jumua2
	}
	if data.RawData.Jumua3 != nil {
		pd.Jumua3 = *data.RawData.Jumua3
	}

	return pd, nil
}

func timeToMinutes(t string) (int, error) {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format: %s", t)
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

type MarkupResult struct {
	Markup          string `json:"markup"`
	MarkupHalfHoriz string `json:"markup_half_horizontal"`
	MarkupHalfVert  string `json:"markup_half_vertical"`
	MarkupQuadrant  string `json:"markup_quadrant"`
}

func renderAllMarkup(tmpl *template.Template, pd *PrayerDisplay) (*MarkupResult, error) {
	result := &MarkupResult{}
	var err error

	result.Markup, err = renderTemplate(tmpl, "full.html", pd)
	if err != nil {
		return nil, fmt.Errorf("render full: %w", err)
	}
	result.MarkupHalfHoriz, err = renderTemplate(tmpl, "half_horizontal.html", pd)
	if err != nil {
		return nil, fmt.Errorf("render half_horizontal: %w", err)
	}
	result.MarkupHalfVert, err = renderTemplate(tmpl, "half_vertical.html", pd)
	if err != nil {
		return nil, fmt.Errorf("render half_vertical: %w", err)
	}
	result.MarkupQuadrant, err = renderTemplate(tmpl, "quadrant.html", pd)
	if err != nil {
		return nil, fmt.Errorf("render quadrant: %w", err)
	}

	return result, nil
}

func renderTemplate(tmpl *template.Template, name string, data *PrayerDisplay) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
