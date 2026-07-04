package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTinyFishFetchEndpoint     = "https://api.fetch.tinyfish.ai"
	defaultTinyFishFetchFormat       = "markdown"
	defaultTinyFishFetchTTL          = 3600
	defaultTinyFishPerURLTimeoutMS   = 45000
	defaultTinyFishHTTPTimeoutBuffer = 15 * time.Second
)

type TinyFishClient struct {
	apiKey          string
	endpoint        string
	format          string
	ttl             *int
	perURLTimeoutMS int
	httpClient      *http.Client
}

type TinyFishFetchedPage struct {
	URL           string
	FinalURL      string
	Title         *string
	Description   *string
	Language      *string
	Author        *string
	PublishedDate *string
	Text          string
	Format        string
}

type TinyFishFetchError struct {
	URL    string `json:"url"`
	Error  string `json:"error"`
	Status *int   `json:"status,omitempty"`
}

type tinyFishFetchRequest struct {
	URLs            []string `json:"urls"`
	Format          string   `json:"format,omitempty"`
	TTL             *int     `json:"ttl,omitempty"`
	PerURLTimeoutMS int      `json:"per_url_timeout_ms,omitempty"`
}

type tinyFishFetchResponse struct {
	Results []tinyFishFetchResult `json:"results"`
	Errors  []TinyFishFetchError  `json:"errors"`
}

type tinyFishFetchResult struct {
	URL           string          `json:"url"`
	FinalURL      string          `json:"final_url"`
	Title         *string         `json:"title"`
	Description   *string         `json:"description"`
	Language      *string         `json:"language"`
	Author        *string         `json:"author"`
	PublishedDate *string         `json:"published_date"`
	Text          json.RawMessage `json:"text"`
	Format        string          `json:"format"`
}

func NewTinyFishClientFromEnv() (*TinyFishClient, error) {
	apiKey := strings.TrimSpace(os.Getenv("TINYFISH_API_KEY"))
	if apiKey == "" {
		return nil, nil
	}

	endpoint := strings.TrimSpace(os.Getenv("TINYFISH_FETCH_ENDPOINT"))
	if endpoint == "" {
		endpoint = defaultTinyFishFetchEndpoint
	}

	format := strings.TrimSpace(os.Getenv("TINYFISH_FETCH_FORMAT"))
	if format == "" {
		format = defaultTinyFishFetchFormat
	}
	if format != "markdown" && format != "html" && format != "json" {
		return nil, fmt.Errorf("invalid TINYFISH_FETCH_FORMAT %q", format)
	}

	perURLTimeoutMS := defaultTinyFishPerURLTimeoutMS
	if raw := strings.TrimSpace(os.Getenv("TINYFISH_FETCH_TIMEOUT_MS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 110000 {
			return nil, fmt.Errorf("invalid TINYFISH_FETCH_TIMEOUT_MS %q", raw)
		}
		perURLTimeoutMS = parsed
	}

	ttl := defaultTinyFishFetchTTL
	if raw := strings.TrimSpace(os.Getenv("TINYFISH_FETCH_TTL")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return nil, fmt.Errorf("invalid TINYFISH_FETCH_TTL %q", raw)
		}
		ttl = parsed
	}

	return &TinyFishClient{
		apiKey:          apiKey,
		endpoint:        endpoint,
		format:          format,
		ttl:             &ttl,
		perURLTimeoutMS: perURLTimeoutMS,
		httpClient: &http.Client{
			Timeout: time.Duration(perURLTimeoutMS)*time.Millisecond + defaultTinyFishHTTPTimeoutBuffer,
		},
	}, nil
}

func NewTinyFishClient(apiKey, endpoint, format string, ttl *int, perURLTimeoutMS int, httpClient *http.Client) *TinyFishClient {
	if endpoint == "" {
		endpoint = defaultTinyFishFetchEndpoint
	}
	if format == "" {
		format = defaultTinyFishFetchFormat
	}
	if perURLTimeoutMS < 1 {
		perURLTimeoutMS = defaultTinyFishPerURLTimeoutMS
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: time.Duration(perURLTimeoutMS)*time.Millisecond + defaultTinyFishHTTPTimeoutBuffer,
		}
	}
	return &TinyFishClient{
		apiKey:          strings.TrimSpace(apiKey),
		endpoint:        endpoint,
		format:          format,
		ttl:             ttl,
		perURLTimeoutMS: perURLTimeoutMS,
		httpClient:      httpClient,
	}
}

func (c *TinyFishClient) FetchContent(ctx context.Context, url string) (*TinyFishFetchedPage, error) {
	pages, fetchErrors, err := c.FetchContents(ctx, []string{url})
	if err != nil {
		return nil, err
	}
	if page, ok := pages[url]; ok {
		return &page, nil
	}
	for _, fetchErr := range fetchErrors {
		if fetchErr.URL == url {
			if fetchErr.Status != nil {
				return nil, fmt.Errorf("tinyfish fetch failed for %s: %s (%d)", url, fetchErr.Error, *fetchErr.Status)
			}
			return nil, fmt.Errorf("tinyfish fetch failed for %s: %s", url, fetchErr.Error)
		}
	}
	return nil, fmt.Errorf("tinyfish fetch returned no content for %s", url)
}

func (c *TinyFishClient) FetchContents(ctx context.Context, urls []string) (map[string]TinyFishFetchedPage, []TinyFishFetchError, error) {
	if c == nil {
		return nil, nil, errors.New("tinyfish client is not configured")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, nil, errors.New("tinyfish API key is not configured")
	}
	if len(urls) == 0 {
		return nil, nil, errors.New("at least one URL is required")
	}
	if len(urls) > 10 {
		return nil, nil, errors.New("tinyfish fetch supports at most 10 URLs per request")
	}

	body, err := json.Marshal(tinyFishFetchRequest{
		URLs:            urls,
		Format:          c.format,
		TTL:             c.ttl,
		PerURLTimeoutMS: c.perURLTimeoutMS,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("encode tinyfish request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create tinyfish request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("call tinyfish fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, nil, fmt.Errorf("tinyfish fetch returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var decoded tinyFishFetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, nil, fmt.Errorf("decode tinyfish response: %w", err)
	}

	pages := make(map[string]TinyFishFetchedPage, len(decoded.Results))
	for _, result := range decoded.Results {
		text, err := tinyFishTextToString(result.Text)
		if err != nil {
			return nil, decoded.Errors, fmt.Errorf("decode tinyfish text for %s: %w", result.URL, err)
		}
		pages[result.URL] = TinyFishFetchedPage{
			URL:           result.URL,
			FinalURL:      result.FinalURL,
			Title:         result.Title,
			Description:   result.Description,
			Language:      result.Language,
			Author:        result.Author,
			PublishedDate: result.PublishedDate,
			Text:          text,
			Format:        result.Format,
		}
	}

	return pages, decoded.Errors, nil
}

func tinyFishTextToString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}

	var compacted bytes.Buffer
	if err := json.Compact(&compacted, raw); err != nil {
		return "", err
	}
	return compacted.String(), nil
}
