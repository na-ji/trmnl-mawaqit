package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Calendar is [12]map[string][]string — list of months, each month is a map
// from day number (string like "1", "2", ...) to 6 prayer time strings.
type MawaqitResponse struct {
	RawData struct {
		Name     string                       `json:"name"`
		Calendar []map[string]json.RawMessage `json:"calendar"`
		Jumua    string                       `json:"jumua"`
		Jumua2   *string                      `json:"jumua2"`
		Jumua3   *string                      `json:"jumua3"`
	} `json:"rawdata"`
}

// GetDayTimes returns the 6 prayer times for a given month (0-indexed) and day (1-indexed).
func (r *MawaqitResponse) GetDayTimes(month, day int) ([]string, error) {
	if month < 0 || month >= len(r.RawData.Calendar) {
		return nil, fmt.Errorf("month %d out of range (calendar has %d months)", month, len(r.RawData.Calendar))
	}
	dayStr := fmt.Sprintf("%d", day)
	raw, ok := r.RawData.Calendar[month][dayStr]
	if !ok {
		return nil, fmt.Errorf("day %d not found in month %d", day, month)
	}
	var times []string
	if err := json.Unmarshal(raw, &times); err != nil {
		return nil, fmt.Errorf("parse times for month %d day %d: %w", month, day, err)
	}
	if len(times) < 6 {
		return nil, fmt.Errorf("expected 6 prayer times, got %d", len(times))
	}
	return times, nil
}

type cacheEntry struct {
	data      *MawaqitResponse
	expiresAt time.Time
}

type MawaqitClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	cache      map[string]cacheEntry
	nowFunc    func() time.Time
}

func NewMawaqitClient(baseURL string) *MawaqitClient {
	return &MawaqitClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      make(map[string]cacheEntry),
		nowFunc:    time.Now,
	}
}

// ishaExpiry computes the expiry time for today's prayer cache based on Isha time.
// The cache expires at Isha time in the given timezone, so that after the last
// prayer of the day, the next fetch retrieves tomorrow's data.
// Falls back to 1 hour from now if Isha time cannot be determined.
func (c *MawaqitClient) ishaExpiry(data *MawaqitResponse, timezone string) time.Time {
	now := c.nowFunc()
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return now.Add(time.Hour)
	}
	nowLocal := now.In(loc)
	month := int(nowLocal.Month()) - 1
	day := nowLocal.Day()

	times, err := data.GetDayTimes(month, day)
	if err != nil || len(times) < 6 {
		return now.Add(time.Hour)
	}

	// Isha is the 6th prayer (index 5)
	ishaMinutes, err := timeToMinutes(times[5])
	if err != nil {
		return now.Add(time.Hour)
	}

	ishaTime := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(),
		ishaMinutes/60, ishaMinutes%60, 0, 0, loc)

	if nowLocal.After(ishaTime) {
		// Isha has passed — expire immediately so tomorrow's data is fetched
		return now
	}
	return ishaTime.In(time.UTC)
}

// GetMosqueData returns mosque prayer data, using a cache that expires after Isha time.
// The timezone parameter is needed to compute when Isha occurs in the mosque's local time.
func (c *MawaqitClient) GetMosqueData(slug string, timezone string) (*MawaqitResponse, error) {
	now := c.nowFunc()
	c.mu.RLock()
	if entry, ok := c.cache[slug]; ok && now.Before(entry.expiresAt) {
		c.mu.RUnlock()
		log.Debug().Str("slug", slug).Msg("mawaqit cache hit")
		return entry.data, nil
	}
	c.mu.RUnlock()

	log.Info().Str("slug", slug).Msg("fetching mawaqit data")
	url := fmt.Sprintf("%s/%s/", c.baseURL, slug)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch mosque data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mawaqit API returned %d: %s", resp.StatusCode, string(body))
	}

	var data MawaqitResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode mawaqit response: %w", err)
	}

	expiry := c.ishaExpiry(&data, timezone)

	c.mu.Lock()
	c.cache[slug] = cacheEntry{data: &data, expiresAt: expiry}
	c.mu.Unlock()

	return &data, nil
}
