// Package nobelprize is the library behind the nobelprize command: the HTTP
// client, pacing, and the typed data models for the Nobel Prize API.
//
// The official Nobel Prize API at api.nobelprize.org is open: no API key,
// no auth. This package wraps it with a rate-limited, retry-capable client
// that the kit operations consume.
package nobelprize

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Host is the base hostname for the Nobel Prize API.
const Host = "api.nobelprize.org"

const (
	baseURL          = "https://" + Host + "/2.1"
	DefaultUserAgent = "nobelprize/dev (+https://github.com/tamnd/nobelprize-cli)"
)

// categoryMap translates human-friendly category names to the API's short codes.
var categoryMap = map[string]string{
	"physics":    "phy",
	"chemistry":  "che",
	"medicine":   "med",
	"literature": "lit",
	"peace":      "pea",
	"economics":  "eco",
}

// knownShortCodes is the set of valid API short codes.
var knownShortCodes = map[string]bool{
	"phy": true,
	"che": true,
	"med": true,
	"lit": true,
	"pea": true,
	"eco": true,
}

// resolveCategory maps a user-supplied category string (full or short) to the
// short code accepted by the API. Returns "" when cat is empty.
func resolveCategory(cat string) string {
	if cat == "" {
		return ""
	}
	if code, ok := categoryMap[cat]; ok {
		return code
	}
	if knownShortCodes[cat] {
		return cat
	}
	// pass through; the API will reject if invalid
	return cat
}

// --- output types ---

// Prize is a Nobel Prize record returned to kit consumers.
type Prize struct {
	Year       string   `json:"year"      kit:"id"`
	Category   string   `json:"category"`
	Motivation string   `json:"motivation"`
	Laureates  []string `json:"laureates"`
}

// Laureate is a Nobel laureate record returned to kit consumers.
type Laureate struct {
	ID       string `json:"id"       kit:"id"`
	Name     string `json:"name"`
	Born     string `json:"born"`
	Country  string `json:"country"`
	Category string `json:"category"`
	Year     string `json:"year"`
}

// --- wire types ---

type wirePrizesResponse struct {
	NobelPrizes []wirePrize `json:"nobelPrizes"`
	Meta        wireMeta    `json:"meta"`
}

type wirePrize struct {
	AwardYear        string         `json:"awardYear"`
	CategoryFullName wireMultiLang  `json:"categoryFullName"`
	Category         wireMultiLang  `json:"category"`
	Motivation       *wireMultiLang `json:"motivation"`
	Laureates        []wireLaureate `json:"laureates"`
}

type wireLaureatesResponse struct {
	Laureates []wireLaureateFull `json:"laureates"`
	Meta      wireMeta           `json:"meta"`
}

type wireLaureateFull struct {
	ID          string              `json:"id"`
	FullName    wireMultiLang       `json:"fullName"`
	KnownName   *wireMultiLang      `json:"knownName"`
	Birth       *wireBirth          `json:"birth"`
	NobelPrizes []wireLaureatePrize `json:"nobelPrizes"`
}

type wireLaureatePrize struct {
	AwardYear        string         `json:"awardYear"`
	CategoryFullName wireMultiLang  `json:"categoryFullName"`
	Motivation       *wireMultiLang `json:"motivation"`
}

type wireLaureate struct {
	ID        string         `json:"id"`
	FullName  wireMultiLang  `json:"fullName"`
	KnownName *wireMultiLang `json:"knownName"`
}

type wireMultiLang struct {
	En string `json:"en"`
	Se string `json:"se"`
	No string `json:"no"`
}

type wireBirth struct {
	Date  string        `json:"date"`
	Place wireBirthPlace `json:"place"`
}

type wireBirthPlace struct {
	Country *wireMultiLang `json:"country"`
}

type wireMeta struct {
	Count int `json:"count"`
	Total int `json:"total"`
}

// firstLang returns m.En if non-empty, else m.Se, else m.No.
func firstLang(m wireMultiLang) string {
	if m.En != "" {
		return m.En
	}
	if m.Se != "" {
		return m.Se
	}
	return m.No
}

// --- Config & Client ---

// Config holds constructor parameters for Client.
type Config struct {
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for the Nobel Prize API.
func DefaultConfig() Config {
	return Config{
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   3,
		Timeout:   15 * time.Second,
	}
}

// Client is a rate-limited HTTP client for the Nobel Prize API.
type Client struct {
	cfg  Config
	http *http.Client
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// ListPrizes fetches Nobel prizes, optionally filtered by category and/or year.
// GET /nobelPrizes?format=json&sort=desc&limit={n}[&nobelPrizeCategory={cat}][&nobelPrizeYear={year}]
func (c *Client) ListPrizes(ctx context.Context, category, year string, limit int) ([]Prize, error) {
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("format", "json")
	q.Set("sort", "desc")
	q.Set("limit", fmt.Sprintf("%d", limit))
	if cat := resolveCategory(category); cat != "" {
		q.Set("nobelPrizeCategory", cat)
	}
	if year != "" {
		q.Set("nobelPrizeYear", year)
	}
	rawURL := baseURL + "/nobelPrizes?" + q.Encode()

	var resp wirePrizesResponse
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return nil, fmt.Errorf("list prizes: %w", err)
	}

	out := make([]Prize, 0, len(resp.NobelPrizes))
	for _, wp := range resp.NobelPrizes {
		p := Prize{
			Year:     wp.AwardYear,
			Category: firstLang(wp.CategoryFullName),
		}
		if wp.Motivation != nil {
			p.Motivation = firstLang(*wp.Motivation)
		}
		for _, wl := range wp.Laureates {
			name := ""
			if wl.KnownName != nil {
				name = firstLang(*wl.KnownName)
			}
			if name == "" {
				name = firstLang(wl.FullName)
			}
			if name != "" {
				p.Laureates = append(p.Laureates, name)
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// SearchLaureates fetches Nobel laureates, optionally filtered by name and/or category.
// GET /laureates?format=json&limit={n}[&name={name}][&nobelPrizeCategory={cat}]
func (c *Client) SearchLaureates(ctx context.Context, name, category string, limit int) ([]Laureate, error) {
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("format", "json")
	q.Set("limit", fmt.Sprintf("%d", limit))
	if name != "" {
		q.Set("name", name)
	}
	if cat := resolveCategory(category); cat != "" {
		q.Set("nobelPrizeCategory", cat)
	}
	rawURL := baseURL + "/laureates?" + q.Encode()

	var resp wireLaureatesResponse
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return nil, fmt.Errorf("search laureates: %w", err)
	}

	out := make([]Laureate, 0, len(resp.Laureates))
	for _, wl := range resp.Laureates {
		l := Laureate{
			ID: wl.ID,
		}
		// Name: prefer knownName
		if wl.KnownName != nil && firstLang(*wl.KnownName) != "" {
			l.Name = firstLang(*wl.KnownName)
		} else {
			l.Name = firstLang(wl.FullName)
		}
		// Birth info
		if wl.Birth != nil {
			if len(wl.Birth.Date) >= 10 {
				l.Born = wl.Birth.Date[:10]
			} else {
				l.Born = wl.Birth.Date
			}
			if wl.Birth.Place.Country != nil {
				l.Country = firstLang(*wl.Birth.Place.Country)
			}
		}
		// First prize
		if len(wl.NobelPrizes) > 0 {
			l.Category = firstLang(wl.NobelPrizes[0].CategoryFullName)
			l.Year = wl.NobelPrizes[0].AwardYear
		}
		out = append(out, l)
	}
	return out, nil
}

// getJSON fetches a URL and JSON-decodes into v.
func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

// get fetches a URL with retries on transient errors.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt) * 500 * time.Millisecond
			if wait > 5*time.Second {
				wait = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}
