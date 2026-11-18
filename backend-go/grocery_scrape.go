package main

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Grocery promo scraper (ported from rumahbot): pulls product names and
// prices from public promo pages of Indonesian grocery stores so the weekly
// meal plan can prefer what is on sale. Off unless MEAL_PROMO_STORES lists
// stores ("superindo", "astro"; comma-separated). Results replace the
// grocery_promos table; a scrape runs before a scheduled meal plan post when
// the stored promos are older than promoMaxAge. Scraping is best effort:
// pages that fail or change shape just yield no promos (the plan still works).

const promoMaxAge = 20 * time.Hour

var promoHTTPClient = &http.Client{Timeout: 15 * time.Second}

// promoFetch is swapped in tests.
var promoFetch = fetchPromoPage

func (app *App) promoStores() []string {
	var out []string
	for _, s := range strings.Split(os.Getenv("MEAL_PROMO_STORES"), ",") {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "superindo" || s == "astro" {
			out = append(out, s)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func fetchPromoPage(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en;q=0.8")
	resp, err := promoHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return "", &promoHTTPError{resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return string(b), err
}

type promoHTTPError struct{ code int }

func (e *promoHTTPError) Error() string { return "HTTP " + http.StatusText(e.code) }

var (
	altRe    = regexp.MustCompile(`alt="([^"]*)"`)
	priceRe  = regexp.MustCompile(`Rp\s*[\d.,]+`)
	ldJSONRe = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
)

var superindoTerms = []string{"ayam", "daging", "ikan", "telur", "udang", "tahu", "tempe", "susu",
	"bayam", "kangkung", "sawi", "brokoli", "wortel", "bawang", "cabe",
	"beras", "minyak", "gula", "tepung", "mie", "roti", "sosis", "nugget",
	"sabun", "shampo", "pampers", "diaper", "tissue", "sarden", "kornet",
	"keju", "mentega", "coklat", "kopi", "teh", "sirup", "kecap",
	"saos", "bumbu", "kaldu", "minuman", "makanan", "snack", "cemilan"}

// parseAltPromos pairs <img alt> product names with Rp prices by position
// (rumahbot's heuristic; prices may be misaligned on some layouts).
func parseAltPromos(page, store, link string, skip map[string]bool, requireTerm bool) []mealPromo {
	names := altRe.FindAllStringSubmatch(page, 30)
	prices := priceRe.FindAllString(page, -1)
	var out []mealPromo
	for i, m := range names {
		name := strings.TrimSpace(html.UnescapeString(m[1]))
		low := strings.ToLower(name)
		if len(name) < 3 || skip[low] {
			continue
		}
		if requireTerm && !containsAnyWord(low, superindoTerms) && !strings.Contains(name, " ") {
			continue
		}
		price := "Promo"
		if i < len(prices) {
			price = strings.Join(strings.Fields(prices[i]), "")
		}
		out = append(out, mealPromo{Store: store, Name: name, Price: price, Link: link})
	}
	return out
}

func scrapeSuperindo(ctx context.Context) []mealPromo {
	skip := map[string]bool{"logo": true, "superindo": true, "super indo": true, "promo": true, "promosi": true, "icon": true, "banner": true}
	var items []mealPromo
	for _, url := range []string{"https://www.superindo.co.id/promosi/super-hebat", "https://www.superindo.co.id/promosi"} {
		page, err := promoFetch(ctx, url)
		if err != nil {
			log.Printf("promo scrape: superindo %s: %v", url, err)
			continue
		}
		items = append(items, parseAltPromos(page, "Superindo", "https://www.superindo.co.id/katalog-promosi", skip, true)...)
		if len(items) >= 10 {
			break
		}
	}
	if len(items) > 20 {
		items = items[:20]
	}
	return items
}

func scrapeAstro(ctx context.Context) []mealPromo {
	skip := map[string]bool{"astro": true, "logo": true, "promo": true, "promosi": true, "banner": true}
	for _, url := range []string{"https://astro.id/promo", "https://astro.id"} {
		page, err := promoFetch(ctx, url)
		if err != nil {
			log.Printf("promo scrape: astro %s: %v", url, err)
			continue
		}
		var items []mealPromo
		if m := ldJSONRe.FindStringSubmatch(page); m != nil {
			var data []map[string]any
			if json.Unmarshal([]byte(m[1]), &data) == nil {
				for _, it := range data {
					if t, _ := it["@type"].(string); t != "Product" && t != "Offer" {
						continue
					}
					name, _ := it["name"].(string)
					price := "Promo"
					if offers, ok := it["offers"].(map[string]any); ok && offers["price"] != nil {
						price = "Rp" + strings.TrimSpace(strings.Trim(jsonScalar(offers["price"]), `"`))
					}
					if name != "" {
						items = append(items, mealPromo{Store: "Astro", Name: name, Price: price, Link: "https://astro.id/promo"})
					}
				}
			}
		}
		if len(items) == 0 {
			items = parseAltPromos(page, "Astro", "https://astro.id/promo", skip, false)
		}
		if len(items) > 0 {
			if len(items) > 20 {
				items = items[:20]
			}
			return items
		}
	}
	return nil
}

func jsonScalar(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// refreshPromosIfStale scrapes when stores are configured and the stored
// promos are older than promoMaxAge (or missing). Never returns an error:
// a failed scrape keeps the previous promos.
func (app *App) refreshPromosIfStale(ctx context.Context, now time.Time) {
	stores := app.promoStores()
	if len(stores) == 0 {
		return
	}
	var newest *time.Time
	_ = app.DB.QueryRow(ctx, "SELECT max(scraped_at) FROM grocery_promos").Scan(&newest)
	if newest != nil && now.Sub(*newest) < promoMaxAge {
		return
	}
	// An empty marker row would be wrong data; instead remember the attempt
	// in meta so a dead site isn't hammered every minute.
	var lastTry string
	_ = app.DB.QueryRow(ctx, "SELECT value FROM meta WHERE key='grocery_promos_attempt'").Scan(&lastTry)
	if t, err := time.Parse(time.RFC3339, lastTry); err == nil && now.Sub(t) < time.Hour {
		return
	}
	_, _ = app.DB.Exec(ctx, `INSERT INTO meta (key, value) VALUES ('grocery_promos_attempt', $1)
	  ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, now.UTC().Format(time.RFC3339))
	if n, err := app.scrapePromos(ctx, stores); err != nil {
		log.Printf("promo scrape: %v", err)
	} else {
		log.Printf("promo scrape: stored %d promos", n)
	}
}

// scrapePromos scrapes the given stores and replaces grocery_promos when at
// least one item was found.
func (app *App) scrapePromos(ctx context.Context, stores []string) (int, error) {
	var items []mealPromo
	for _, s := range stores {
		switch s {
		case "superindo":
			items = append(items, scrapeSuperindo(ctx)...)
		case "astro":
			items = append(items, scrapeAstro(ctx)...)
		}
	}
	if len(items) == 0 {
		return 0, nil
	}
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "DELETE FROM grocery_promos"); err != nil {
		return 0, err
	}
	for _, it := range items {
		if _, err := tx.Exec(ctx, "INSERT INTO grocery_promos (store, name, price, link) VALUES ($1,$2,$3,$4)", it.Store, it.Name, it.Price, it.Link); err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit(ctx)
}
