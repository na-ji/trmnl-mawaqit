package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
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
	fetchedAt time.Time
}

type MawaqitClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	cache      map[string]cacheEntry
}

func NewMawaqitClient(baseURL string) *MawaqitClient {
	return &MawaqitClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      make(map[string]cacheEntry),
	}
}

func (c *MawaqitClient) GetMosqueData(slug string) (*MawaqitResponse, error) {
	c.mu.RLock()
	if entry, ok := c.cache[slug]; ok && time.Since(entry.fetchedAt) < time.Hour {
		c.mu.RUnlock()
		return entry.data, nil
	}
	c.mu.RUnlock()

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

	c.mu.Lock()
	c.cache[slug] = cacheEntry{data: &data, fetchedAt: time.Now()}
	c.mu.Unlock()

	return &data, nil
}
