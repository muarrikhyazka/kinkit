package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

// fx rates (12h file cache, no new deps)

const fxCacheTTL = 12 * time.Hour
const fxAPIURL = "https://open.er-api.com/v6/latest/IDR"

type fxCache struct {
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
	FetchedAt *float64           `json:"fetched_at"`
}

func fxCachePath() string {
	return getenv("FX_CACHE_PATH", "/app/data/fx_rates.json")
}

func loadFxCacheFile(path string) *fxCache {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c fxCache
	if err := json.Unmarshal(b, &c); err != nil {
		return nil
	}
	return &c
}

func saveFxCacheFile(path string, c *fxCache) {
	_ = os.MkdirAll(dirOf(path), 0o755)
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	return path[:i]
}

// getFxRates ports get_fx_rates(): file-cached rates, refreshed at most once
// per 12h via stdlib net/http, stale cache (or nil) beats a hard failure.
func getFxRates() *fxCache {
	path := fxCachePath()
	cached := loadFxCacheFile(path)
	now := float64(time.Now().UTC().Unix())
	if cached != nil && cached.FetchedAt != nil && now-*cached.FetchedAt < fxCacheTTL.Seconds() {
		return cached
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fxAPIURL)
	if err != nil {
		return cached
	}
	defer resp.Body.Close()
	var data struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return cached
	}
	fetchedAt := now
	fresh := &fxCache{Base: "IDR", Rates: data.Rates, FetchedAt: &fetchedAt}
	saveFxCacheFile(path, fresh)
	return fresh
}

func (app *App) handleFxRates(w http.ResponseWriter, r *http.Request) {
	cached := getFxRates()
	if cached == nil {
		writeJSON(w, http.StatusOK, map[string]any{"base": "IDR", "rates": map[string]float64{"IDR": 1}, "fetched_at": nil})
		return
	}
	writeJSON(w, http.StatusOK, cached)
}
