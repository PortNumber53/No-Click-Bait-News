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

	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

const (
	defaultLLMBaseURL          = "https://api.openai.com/v1"
	defaultLLMTemperature      = 0.2
	defaultLLMMaxTokens        = 3000
	defaultLLMHTTPTimeout      = 5 * time.Minute
	ArticleRewriteAgentVersion = 3
)

type ArticleRewriter struct {
	baseURL     string
	model       string
	temperature float64
	maxTokens   int
	httpClient  *http.Client
	control     *RewriteControl
}

// LLMProviderError describes an HTTP failure returned by the rewrite provider.
type LLMProviderError struct {
	StatusCode int
	KeySlot    int
	RetryAfter time.Duration
	RetryAt    time.Time
	Body       string
}

func (e *LLMProviderError) Error() string {
	return fmt.Sprintf("LLM rewrite API returned HTTP %d (key_slot=%d): %s", e.StatusCode, e.KeySlot, e.Body)
}

// IsLLMRateLimitError identifies provider and local-circuit rate limiting.
func IsLLMRateLimitError(err error) (*LLMProviderError, bool) {
	var providerErr *LLMProviderError
	if !errors.As(err, &providerErr) || providerErr.StatusCode != http.StatusTooManyRequests {
		return nil, false
	}
	return providerErr, true
}

type ArticleRewriteResult struct {
	Content       string   `json:"content"`
	Categories    []string `json:"categories"`
	Title         string   `json:"title,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	BiasLabel     string   `json:"bias_label"`
	BiasReasoning string   `json:"bias_reasoning"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func NewArticleRewriterFromEnv() (*ArticleRewriter, error) {
	rewriters, err := NewArticleRewritersFromEnv()
	if err != nil || len(rewriters) == 0 {
		return nil, err
	}
	return rewriters[0], nil
}

func NewArticleRewritersFromEnv() ([]*ArticleRewriter, error) {
	apiKeysRaw := strings.TrimSpace(os.Getenv("LLM_BATCH_API_KEYS"))
	modelsRaw := strings.TrimSpace(os.Getenv("LLM_MODELS"))
	if modelsRaw == "" {
		modelsRaw = strings.TrimSpace(os.Getenv("LLM_MODEL"))
	}
	if apiKeysRaw == "" && modelsRaw == "" {
		return nil, nil
	}
	if apiKeysRaw == "" {
		return nil, errors.New("LLM_BATCH_API_KEYS is required when LLM_MODEL or LLM_MODELS is set")
	}
	if modelsRaw == "" {
		return nil, errors.New("LLM_MODEL or LLM_MODELS is required when LLM_BATCH_API_KEYS is set")
	}
	apiKeys := uniqueNonEmptyStrings(strings.Split(apiKeysRaw, ","))
	if len(apiKeys) == 0 {
		return nil, errors.New("LLM_BATCH_API_KEYS must include at least one key")
	}
	models := uniqueNonEmptyStrings(strings.Split(modelsRaw, ","))
	if len(models) == 0 {
		return nil, errors.New("at least one LLM model is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("LLM_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultLLMBaseURL
	}

	temperature := defaultLLMTemperature
	if raw := strings.TrimSpace(os.Getenv("LLM_TEMPERATURE")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed < 0 || parsed > 2 {
			return nil, fmt.Errorf("invalid LLM_TEMPERATURE %q", raw)
		}
		temperature = parsed
	}

	maxTokens := defaultLLMMaxTokens
	if raw := strings.TrimSpace(os.Getenv("LLM_MAX_TOKENS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return nil, fmt.Errorf("invalid LLM_MAX_TOKENS %q", raw)
		}
		maxTokens = parsed
	}
	startsPerInterval, err := positiveIntEnv("LLM_REWRITE_STARTS_PER_INTERVAL", defaultRewriteStartsPerInterval)
	if err != nil {
		return nil, err
	}
	intervalSeconds, err := positiveIntEnv("LLM_REWRITE_INTERVAL_SECONDS", int(defaultRewriteInterval/time.Second))
	if err != nil {
		return nil, err
	}
	backoffBaseSeconds, err := positiveIntEnv("LLM_REWRITE_429_BASE_SECONDS", int(defaultRewrite429Base/time.Second))
	if err != nil {
		return nil, err
	}
	backoffMaxSeconds, err := positiveIntEnv("LLM_REWRITE_429_MAX_SECONDS", int(defaultRewrite429Max/time.Second))
	if err != nil {
		return nil, err
	}
	if backoffMaxSeconds < backoffBaseSeconds {
		return nil, errors.New("LLM_REWRITE_429_MAX_SECONDS must be greater than or equal to LLM_REWRITE_429_BASE_SECONDS")
	}
	control := newRewriteControl(apiKeys, startsPerInterval, time.Duration(intervalSeconds)*time.Second, time.Duration(backoffBaseSeconds)*time.Second, time.Duration(backoffMaxSeconds)*time.Second)

	rewriters := make([]*ArticleRewriter, 0, len(models))
	for _, model := range models {
		rewriters = append(rewriters, newArticleRewriter(control, baseURL, model, temperature, maxTokens, nil))
	}
	return rewriters, nil
}

func positiveIntEnv(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("invalid %s %q", key, raw)
	}
	return parsed, nil
}

func uniqueNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func NewArticleRewriter(apiKey, baseURL, model string, temperature float64, maxTokens int, httpClient *http.Client) *ArticleRewriter {
	control := newRewriteControl(uniqueNonEmptyStrings([]string{apiKey}), defaultRewriteStartsPerInterval, defaultRewriteInterval, defaultRewrite429Base, defaultRewrite429Max)
	return newArticleRewriter(control, baseURL, model, temperature, maxTokens, httpClient)
}

func newArticleRewriter(control *RewriteControl, baseURL, model string, temperature float64, maxTokens int, httpClient *http.Client) *ArticleRewriter {
	if baseURL == "" {
		baseURL = defaultLLMBaseURL
	}
	if temperature < 0 {
		temperature = defaultLLMTemperature
	}
	if maxTokens < 1 {
		maxTokens = defaultLLMMaxTokens
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultLLMHTTPTimeout}
	}
	return &ArticleRewriter{
		baseURL:     strings.TrimRight(baseURL, "/"),
		model:       strings.TrimSpace(model),
		temperature: temperature,
		maxTokens:   maxTokens,
		httpClient:  httpClient,
		control:     control,
	}
}

func (r *ArticleRewriter) Control() *RewriteControl {
	if r == nil {
		return nil
	}
	return r.control
}

func (r *ArticleRewriter) AgentVersion() int {
	if r == nil {
		return 0
	}
	return ArticleRewriteAgentVersion
}

func (r *ArticleRewriter) Model() string {
	if r == nil {
		return ""
	}
	return r.model
}

func (r *ArticleRewriter) RewriteArticle(ctx context.Context, title, sourceURL, originalMarkdown string) (ArticleRewriteResult, error) {
	if r == nil {
		return ArticleRewriteResult{}, errors.New("article rewriter is not configured")
	}
	if r.control == nil || r.model == "" {
		return ArticleRewriteResult{}, errors.New("article rewriter API key and model are required")
	}
	if retryAt, open := r.control.cooldownUntil(r.control.now()); open {
		return ArticleRewriteResult{}, &LLMProviderError{
			StatusCode: http.StatusTooManyRequests,
			RetryAt:    retryAt,
			Body:       "shared rate-limit circuit breaker is open",
		}
	}
	apiKey, keySlot, err := r.control.nextAPIKey()
	if err != nil {
		return ArticleRewriteResult{}, err
	}

	prompt := fmt.Sprintf(`Rewrite this news article in clean markdown for NoClickBait News and classify it.

Agent version: %d

Rules:
- Keep the facts, names, numbers, dates, quotes, and attributions accurate.
- Be concise and direct.
- Start with the most important information. Do not build suspense or delay the main point.
- Remove rambling, filler, teasers, promotional language, and "great reveal" structure.
- Remove text unrelated to the news subject, including navigation copy, ad/link copy, newsletter prompts, social sharing text, recommendations, and boilerplate that is not part of the article.
- Preserve useful headings, bullet lists, and links when they help comprehension.
- Use Markdown only. Do not return HTML tags.
- Do not add facts, claims, opinions, analysis, or context that is not in the original.
- Assign 1 to 3 categories that best fit the article.
- Categories must come from this exact list: %s.
- Assess bias in the original article, not in your rewrite. Use specific textual evidence and do not infer the author's private intent.
- bias_label must be exactly one of: No clear bias, Left-leaning, Right-leaning, Mixed political framing, Loaded or sensational framing, Source imbalance, Promotional or advocacy framing, Unclear.
- bias_reasoning must be a neutral 2-4 sentence explanation of the evidence for the label. If there is not enough evidence, use Unclear and explain what is missing.
- Return only valid JSON. Do not wrap it in markdown fences.

JSON shape:
{
	"title": "direct factual headline",
	"summary": "2-4 sentence factual summary",
  "content": "rewritten article markdown",
  "categories": ["Business", "Technology"],
  "bias_label": "No clear bias",
  "bias_reasoning": "The article attributes its factual claims and avoids loaded wording. It presents no clear political or promotional framing."
}

Title: %s
Source URL: %s

Original markdown:
%s`, r.AgentVersion(), strings.Join(models.AllowedArticleCategories, ", "), title, sourceURL, originalMarkdown)

	body, err := json.Marshal(chatCompletionRequest{
		Model: r.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "You rewrite news articles into direct, concise, factual markdown without clickbait structure.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: r.temperature,
		MaxTokens:   r.maxTokens,
	})
	if err != nil {
		return ArticleRewriteResult{}, fmt.Errorf("encode LLM rewrite request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ArticleRewriteResult{}, fmt.Errorf("create LLM rewrite request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return ArticleRewriteResult{}, fmt.Errorf("call LLM rewrite API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		providerErr := &LLMProviderError{StatusCode: resp.StatusCode, KeySlot: keySlot, Body: strings.TrimSpace(string(responseBody))}
		if resp.StatusCode == http.StatusTooManyRequests {
			providerErr.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), r.control.now())
			providerErr.RetryAt = r.control.recordRateLimit(providerErr.RetryAfter)
		}
		return ArticleRewriteResult{}, providerErr
	}
	r.control.recordSuccess()

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ArticleRewriteResult{}, fmt.Errorf("decode LLM rewrite response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return ArticleRewriteResult{}, errors.New("LLM rewrite response did not include choices")
	}

	raw := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if raw == "" {
		return ArticleRewriteResult{}, errors.New("LLM rewrite response was empty")
	}
	result, err := parseArticleRewriteResult(raw)
	if err != nil {
		return ArticleRewriteResult{}, err
	}
	return result, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return 0
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	return when.Sub(now)
}

func parseArticleRewriteResult(raw string) (ArticleRewriteResult, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result ArticleRewriteResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return ArticleRewriteResult{}, fmt.Errorf("decode LLM rewrite content JSON: %w", err)
	}

	result.Content = stripHTMLMarkup(result.Content)
	result.Title = strings.Join(strings.Fields(stripHTMLMarkup(result.Title)), " ")
	result.Summary = strings.Join(strings.Fields(stripHTMLMarkup(result.Summary)), " ")
	result.BiasLabel = normalizeBiasLabel(result.BiasLabel)
	result.BiasReasoning = strings.Join(strings.Fields(stripHTMLMarkup(result.BiasReasoning)), " ")
	if result.Content == "" {
		return ArticleRewriteResult{}, errors.New("LLM rewrite content was empty")
	}
	result.Categories = models.NormalizeArticleCategories(result.Categories)
	if len(result.Categories) == 0 {
		return ArticleRewriteResult{}, errors.New("LLM rewrite did not include valid categories")
	}
	if result.BiasLabel == "" {
		return ArticleRewriteResult{}, errors.New("LLM rewrite did not include a valid bias label")
	}
	if result.BiasReasoning == "" {
		return ArticleRewriteResult{}, errors.New("LLM rewrite did not include bias reasoning")
	}

	return result, nil
}

func normalizeBiasLabel(value string) string {
	labels := []string{
		"No clear bias",
		"Left-leaning",
		"Right-leaning",
		"Mixed political framing",
		"Loaded or sensational framing",
		"Source imbalance",
		"Promotional or advocacy framing",
		"Unclear",
	}
	value = strings.TrimSpace(value)
	for _, label := range labels {
		if strings.EqualFold(value, label) {
			return label
		}
	}
	return ""
}
