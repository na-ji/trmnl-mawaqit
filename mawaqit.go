package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Official Mawaqit search endpoint. Independent from MAWAQIT_API_BASE which
// points to the v1 mosque-detail API (which may be self-hosted).
const mawaqitSearchURL = "https://mawaqit.net/api/2.0/mosque/search"

type MosqueSearchResult struct {
	Slug         string `json:"slug"`
	Label        string `json:"label"`
	Localisation string `json:"localisation"`
}

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

// midnightExpiry returns midnight local time (start of the next day) in the given timezone.
// Falls back to 1 hour from now if the timezone cannot be loaded.
func (c *MawaqitClient) midnightExpiry(timezone string) time.Time {
	now := c.nowFunc()
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return now.Add(time.Hour)
	}
	nowLocal := now.In(loc)
	midnight := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day()+1,
		0, 0, 0, 0, loc)
	return midnight.In(time.UTC)
}

// GetMosqueData returns mosque prayer data, using a cache that expires at midnight local time.
func (c *MawaqitClient) GetMosqueData(slug string, timezone string) (*MawaqitResponse, error) {
	now := c.nowFunc()
	c.mu.RLock()
	if entry, ok := c.cache[slug]; ok && now.Before(entry.expiresAt) {
		c.mu.RUnlock()
		mawaqitAPICacheHitsTotal.WithLabelValues(slug).Inc()
		log.Debug().Str("slug", slug).Msg("mawaqit cache hit")
		return entry.data, nil
	}
	c.mu.RUnlock()

	mawaqitAPICallsTotal.WithLabelValues(slug).Inc()
	log.Info().Str("slug", slug).Msg("fetching mawaqit data")
	url := fmt.Sprintf("%s/%s/", c.baseURL, slug)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		mawaqitAPIErrorsTotal.WithLabelValues(slug).Inc()
		return nil, fmt.Errorf("fetch mosque data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		mawaqitAPIErrorsTotal.WithLabelValues(slug).Inc()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mawaqit API returned %d: %s", resp.StatusCode, string(body))
	}

	var data MawaqitResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode mawaqit response: %w", err)
	}

	expiry := c.midnightExpiry(timezone)

	c.mu.Lock()
	c.cache[slug] = cacheEntry{data: &data, expiresAt: expiry}
	c.mu.Unlock()

	return &data, nil
}

// SearchMosques queries the official Mawaqit search API and returns a list of
// {slug, label, localisation}. The API returns additional fields which we discard.
func (c *MawaqitClient) SearchMosques(keyword string) ([]MosqueSearchResult, error) {
	q := url.Values{}
	q.Set("word", keyword)
	q.Set("fields", "slug,label,localisation")

	req, err := http.NewRequest(http.MethodGet, mawaqitSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search mosques: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mawaqit search returned %d: %s", resp.StatusCode, string(body))
	}

	var results []MosqueSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	return results, nil
}
