package freedomist

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.exfreedomist.com"

type Client struct {
	token string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

// SearchQuery - request body for /search endpoint
type SearchQuery struct {
	Query        string   `json:"query"`
	Trackers     []string `json:"trackers,omitempty"`
	OrderBy      string   `json:"order_by,omitempty"`
	FilterBySize string   `json:"filter_by_size,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	Offset       int      `json:"offset,omitempty"`
	FullMatch    bool     `json:"full_match,omitempty"`
	Token        string   `json:"token"`
}

type SearchItem struct {
	Rank         int    `json:"rank"`
	Metric       int    `json:"metric"`
	Title        string `json:"title"`
	Tracker      string `json:"tracker"`
	TopicID      int    `json:"topic_id"`
	BoardID      int    `json:"board_id"`
	Seeders      int    `json:"seeders"`
	Leechers     int    `json:"leechers"`
	Downloads    int    `json:"downloads"`
	Size         string `json:"size"`
	PostDatetime string `json:"post_datetime"`
	IndexDate    string `json:"index_date"`
	MagnetKey    string `json:"magnet_key"`
	Status       string `json:"status"`
}

type SearchResponse struct {
	Data       []SearchItem `json:"data"`
	FullMatch  bool         `json:"full_match"`
	Message    string       `json:"message"`
	StatusCode int          `json:"status_code"`
}

type MagnetItem struct {
	MagnetLink string `json:"magnet_link"`
	TopicID    int    `json:"topic_id"`
	Tracker    string `json:"tracker"`
	Rank       string `json:"rank"`
}

type MagnetResponse struct {
	Data       MagnetItem `json:"data"`
	Message    string     `json:"message"`
	StatusCode int        `json:"status_code"`
}

func (c *Client) Search(q SearchQuery) (*SearchResponse, error) {
	q.Token = c.token
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.OrderBy == "" {
		q.OrderBy = "s"
	}

	body, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Post(apiBase+"/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.StatusCode != 0 && result.StatusCode != 200 {
		return nil, fmt.Errorf("freedomist: search error %d: %s", result.StatusCode, result.Message)
	}
	return &result, nil
}

func (c *Client) Magnet(key string) (string, error) {
	if key == "" {
		return "", errors.New("freedomist: empty magnet key")
	}

	url := fmt.Sprintf("%s/magnet/%s?token=%s", apiBase, key, c.token)
	resp, err := c.http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result MagnetResponse
	if err = json.Unmarshal(b, &result); err != nil {
		return "", err
	}
	if result.Data.MagnetLink == "" {
		return "", fmt.Errorf("freedomist: no magnet for key %s", key)
	}
	return result.Data.MagnetLink, nil
}
