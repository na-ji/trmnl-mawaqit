package main

import (
	"bytes"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"sync"
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
	// NextPrayerTime is the absolute time of the next prayer, used for markup cache expiry.
	NextPrayerTime time.Time
}

var names = []string{"Fajr", "Shuruq", "Dohr", "Asr", "Maghrib", "Isha"}

// nowFunc can be overridden in tests; defaults to time.Now.
var nowFunc = time.Now

func buildPrayerDisplay(data *MawaqitResponse, timezone string) (*PrayerDisplay, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	now := nowFunc().In(loc)

	month := int(now.Month()) - 1 // 0-indexed
	day := now.Day()              // 1-indexed

	times, err := data.GetDayTimes(month, day)
	if err != nil {
		return nil, err
	}

	// Determine next prayer
	nowMinutes := now.Hour()*60 + now.Minute()
	nextIdx := -1
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

	// All prayers have passed (after Isha) — switch to tomorrow's times
	afterIsha := nextIdx == -1
	if afterIsha {
		nextIdx = 0 // Fajr is next
		tomorrow := now.AddDate(0, 0, 1)
		tomorrowTimes, err := data.GetDayTimes(int(tomorrow.Month())-1, tomorrow.Day())
		if err == nil {
			times = tomorrowTimes
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

	// Compute the absolute time of the next prayer for cache expiry
	nextMinutes, _ := timeToMinutes(times[nextIdx])
	nextPrayerTime := time.Date(now.Year(), now.Month(), now.Day(),
		nextMinutes/60, nextMinutes%60, 0, 0, loc)
	if afterIsha {
		nextPrayerTime = nextPrayerTime.AddDate(0, 0, 1)
	}

	pd := &PrayerDisplay{
		MosqueName:     data.RawData.Name,
		Prayers:        prayers,
		Jumua:          data.RawData.Jumua,
		NextPrayerTime: nextPrayerTime,
	}
	if data.RawData.Jumua2 != nil {
		pd.Jumua2 = *data.RawData.Jumua2
	}
	if data.RawData.Jumua3 != nil {
		pd.Jumua3 = *data.RawData.Jumua3
	}

	return pd, nil
}

// MarkupCache caches rendered markup per mosque slug, expiring at the next prayer time.
type MarkupCache struct {
	mu      sync.RWMutex
	entries map[string]markupCacheEntry
	nowFunc func() time.Time
}

type markupCacheEntry struct {
	result    *MarkupResult
	expiresAt time.Time
}

func NewMarkupCache() *MarkupCache {
	return &MarkupCache{
		entries: make(map[string]markupCacheEntry),
		nowFunc: time.Now,
	}
}

// Get returns a cached MarkupResult for the given mosque slug, or nil if expired/missing.
func (mc *MarkupCache) Get(mosqueSlug string) *MarkupResult {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	entry, ok := mc.entries[mosqueSlug]
	if !ok || mc.nowFunc().After(entry.expiresAt) {
		return nil
	}
	return entry.result
}

// Set stores a MarkupResult for the given mosque slug, expiring at expiresAt.
func (mc *MarkupCache) Set(mosqueSlug string, result *MarkupResult, expiresAt time.Time) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.entries[mosqueSlug] = markupCacheEntry{result: result, expiresAt: expiresAt}
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
