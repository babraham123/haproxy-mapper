package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Fetcher retrieves a document by URL. httpFetcher is the production adapter;
// tests substitute fixtures.
type Fetcher interface {
	Fetch(url string) (io.ReadCloser, error)
}

type httpFetcher struct {
	client http.Client
}

func HttpFetcherCreate() *httpFetcher {
	return &httpFetcher{
		client: http.Client{
			// Long enough for the GeoLite2 City download, which is tens of MB.
			Timeout: 5 * time.Minute,
		},
	}
}

func (h *httpFetcher) Fetch(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("user-agent", "haproxy-mapper")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

func fetchBytes(f Fetcher, url string) ([]byte, error) {
	body, err := f.Fetch(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

func fetchJSON(f Fetcher, url string, out interface{}) error {
	body, err := fetchBytes(f, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	return nil
}
