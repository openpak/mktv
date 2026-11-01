package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// These mirror the JSON of the mario-kart-8 read API (nex repo, titles/mario-kart-8/api.go).

type Object struct {
	ID        uint64   `json:"id"`
	Owner     string   `json:"owner"`
	Name      string   `json:"name"`
	Size      int64    `json:"size"`
	DataType  int      `json:"data_type"`
	Slot      *int     `json:"slot"`
	Tags      []string `json:"tags"`
	MetaSize  int      `json:"meta_size"`
	MetaHex   string   `json:"meta_hex"`
	Thumbnail *uint64  `json:"thumbnail"`
	Rating    *struct {
		Total int64 `json:"total"`
		Count int64 `json:"count"`
	} `json:"rating"`
	YouTubeID string    `json:"youtube_id"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

type Rank struct {
	Rank    int       `json:"rank"`
	Player  string    `json:"player"`
	TimeMS  int64     `json:"time_ms"`
	Time    string    `json:"time"`
	Updated time.Time `json:"updated"`
}

type Course struct {
	Course int `json:"course"`
	Ranked int `json:"ranked"`
	Ghosts int `json:"ghosts"`
}

type PlayerRank struct {
	Course int    `json:"course"`
	Rank   int    `json:"rank"`
	Of     int    `json:"of"`
	Time   string `json:"time"`
}

type Score struct {
	Rank    int       `json:"rank"`
	Player  string    `json:"player"`
	Score   int64     `json:"score"`
	Team    int       `json:"team"`
	Updated time.Time `json:"updated"`
}

type Tournament struct {
	ID           uint32     `json:"id"`
	Owner        string     `json:"owner"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Code         string     `json:"community_code"`
	Start        *time.Time `json:"start"`
	End          *time.Time `json:"end"`
	Participants int64      `json:"participants"`
	Season       int64      `json:"season"`
	Created      time.Time  `json:"created"`
	Scores       []Score    `json:"scores"`
}

type Page[T any] struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Items  []T `json:"items"`
}

var errNotFound = errors.New("not found")

// apiClient reads the title's API, with a short cache so a busy page costs the game server
// one query per path per few seconds.
type apiClient struct {
	base string
	http *http.Client
	ttl  time.Duration

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	at   time.Time
	body []byte
}

func newAPIClient(base string) *apiClient {
	return &apiClient{base: base, http: &http.Client{Timeout: 6 * time.Second}, ttl: 10 * time.Second, cache: map[string]cached{}}
}

func (c *apiClient) raw(ctx context.Context, path string) ([]byte, error) {
	c.mu.Lock()
	if e, ok := c.cache[path]; ok && time.Since(e.at) < c.ttl {
		c.mu.Unlock()
		return e.body, nil
	}
	c.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, errNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("api %s: %s", path, resp.Status)
	}
	c.mu.Lock()
	if len(c.cache) > 512 {
		c.cache = map[string]cached{}
	}
	c.cache[path] = cached{time.Now(), body}
	c.mu.Unlock()
	return body, nil
}

func get[T any](ctx context.Context, c *apiClient, path string) (T, error) {
	var out T
	b, err := c.raw(ctx, path)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(b, &out)
}

func q(limit, offset int, owner string) string {
	v := url.Values{"limit": {fmt.Sprint(limit)}, "offset": {fmt.Sprint(offset)}}
	if owner != "" {
		v.Set("owner", owner)
	}
	return v.Encode()
}

// media streams an object body (thumbnail or ghost) from the API to w.
func (c *apiClient) media(ctx context.Context, id uint64, w http.ResponseWriter) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/objects/%d/body", c.base, id), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusNotImplemented:
		return errNotFound
	default:
		return fmt.Errorf("api media: %s", resp.Status)
	}
	for _, h := range []string{"Content-Type", "Content-Disposition", "Content-Length", "Cache-Control"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	_, err = io.Copy(w, resp.Body)
	return err
}
