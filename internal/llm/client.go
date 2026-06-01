package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"wohnungssuche/internal/config"
)

type Client struct {
	httpClient       *http.Client
	apiURL           string
	apiKey           string
	model            string
	maxPageTextChars int
}

type ExtractedListing struct {
	Title         string `json:"title"`
	Address       string `json:"address"`
	ColdRent      string `json:"cold_rent"`
	WarmRent      string `json:"warm_rent"`
	Rooms         string `json:"rooms"`
	AreaSQM       string `json:"area_sqm"`
	AvailableFrom string `json:"available_from"`
	Contact       string `json:"contact"`
	Summary       string `json:"summary"`
	ListingURL    string `json:"listing_url"`
}

func New(cfg config.Config) *Client {
	return &Client{
		httpClient:       &http.Client{Timeout: cfg.HTTPTimeout},
		apiURL:           cfg.LLMAPIURL,
		apiKey:           cfg.LLMAPIKey,
		model:            cfg.LLMModel,
		maxPageTextChars: cfg.MaxPageTextChars,
	}
}

func (c *Client) ExtractListing(ctx context.Context, pageURL string, pageTitle string, pageText string) (ExtractedListing, string, error) {
	trimmedText := truncate(pageText, c.maxPageTextChars)

	requestBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Du extrahierst strukturierte Informationen zu Mietwohnungen. Antworte ausschließlich mit einem JSON-Objekt. Wenn ein Feld fehlt, gib einen leeren String zurück.",
			},
			{
				"role":    "user",
				"content": fmt.Sprintf("Extrahiere relevante Informationen zu der verfügbaren Mietwohnung aus folgendem Webseiteninhalt. URL: %s\nSeitentitel: %s\nInhalt:\n%s\n\nJSON-Felder: title, address, cold_rent, warm_rent, rooms, area_sqm, available_from, contact, summary, listing_url", pageURL, pageTitle, trimmedText),
			},
		},
		"temperature": 0.1,
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return ExtractedListing{}, "", fmt.Errorf("marshal llm request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(payload))
	if err != nil {
		return ExtractedListing{}, "", fmt.Errorf("build llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ExtractedListing{}, "", fmt.Errorf("call llm api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExtractedListing{}, "", fmt.Errorf("read llm response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ExtractedListing{}, string(body), fmt.Errorf("llm api returned %s", resp.Status)
	}

	var chatResponse struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chatResponse); err != nil {
		return ExtractedListing{}, string(body), fmt.Errorf("decode llm response: %w", err)
	}

	if len(chatResponse.Choices) == 0 {
		return ExtractedListing{}, string(body), fmt.Errorf("llm response did not contain choices")
	}

	rawContent, err := contentToString(chatResponse.Choices[0].Message.Content)
	if err != nil {
		return ExtractedListing{}, string(body), fmt.Errorf("parse llm message content: %w", err)
	}

	jsonPayload, err := extractJSONObject(rawContent)
	if err != nil {
		return ExtractedListing{}, rawContent, fmt.Errorf("extract json payload: %w", err)
	}

	var extracted ExtractedListing
	if err := json.Unmarshal([]byte(jsonPayload), &extracted); err != nil {
		return ExtractedListing{}, rawContent, fmt.Errorf("decode extracted listing: %w", err)
	}

	return extracted, rawContent, nil
}

func contentToString(content any) (string, error) {
	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			partMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := partMap["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n"), nil
	default:
		return "", fmt.Errorf("unsupported content type %T", content)
	}
}

func extractJSONObject(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("no json object found")
	}

	return trimmed[start : end+1], nil
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}

	return value[:max]
}
