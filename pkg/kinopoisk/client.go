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
	"time"
)

const DefaultBaseURL = "https://api.poiskkino.dev"

type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token:   token,
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
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
