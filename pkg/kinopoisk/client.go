package kinopoisk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://api.poiskkino.dev"

type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client

	mu    sync.Mutex
	cache map[string]any
}

func NewClient(token string) *Client {
	c := &Client{
		Token:   token,
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		cache:   map[string]any{},
	}
	go c.resetLoop()
	return c
}

// Cached returns cached value or calls fetch, stores and returns the result
func (c *Client) Cached(key string, fetch func() (any, error)) (any, error) {
	c.mu.Lock()
	if v, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	v, err := fetch()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[key] = v
	c.mu.Unlock()

	return v, nil
}

// resetLoop clears cache at midnight Moscow time (UTC+3)
func (c *Client) resetLoop() {
	msk := time.FixedZone("MSK", 3*60*60)
	for {
		now := time.Now().In(msk)
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, msk)
		time.Sleep(next.Sub(now))

		c.mu.Lock()
		c.cache = map[string]any{}
		c.mu.Unlock()
	}
}

// GetMovie returns full movie info by KinoPoisk ID
func (c *Client) GetMovie(id int) (*Movie, error) {
	var m Movie
	if err := c.get(fmt.Sprintf("/v1.4/movie/%d", id), nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// SearchMovies searches movies by title string
func (c *Client) SearchMovies(query string, page, limit int) (*SearchResponse, error) {
	params := url.Values{
		"query": {query},
		"page":  {strconv.Itoa(page)},
		"limit": {strconv.Itoa(limit)},
	}
	var resp SearchResponse
	if err := c.get("/v1.4/movie/search", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// QueryMovies - universal search with filters via /v1.5/movie
func (c *Client) QueryMovies(params url.Values) (*CursorResponse, error) {
	var resp CursorResponse
	if err := c.get("/v1.5/movie", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// internals

func (c *Client) get(path string, params url.Values, dst any) error {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}

	u := base + path
	if params != nil {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return fmt.Errorf("kinopoisk: %w", err)
	}
	req.Header.Set("X-API-KEY", c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("kinopoisk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.New("kinopoisk: " + strconv.Itoa(resp.StatusCode) + " " + strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("kinopoisk: decode: %w", err)
	}
	return nil
}
