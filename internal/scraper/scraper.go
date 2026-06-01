package scraper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"gorm.io/datatypes"

	"wohnungssuche/internal/config"
	"wohnungssuche/internal/llm"
)

const availabilityMarker = "Mietwohnungen verfügbar"

type Candidate struct {
	SourceURL        string
	PageTitle        string
	AvailabilityText string
	Headline         string
	Summary          string
	ExtractedData    datatypes.JSON
	LLMRawResponse   string
	Fingerprint      string
}

type Outcome struct {
	Available bool
	Status    string
	Candidate *Candidate
}

type Scraper struct {
	httpClient *http.Client
	llmClient  *llm.Client
	userAgent  string
}

func New(cfg config.Config, llmClient *llm.Client) *Scraper {
	return &Scraper{
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
		llmClient:  llmClient,
		userAgent:  cfg.UserAgent,
	}
}

func (s *Scraper) Scrape(ctx context.Context, targetURL string) (Outcome, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return Outcome{}, fmt.Errorf("build fetch request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return Outcome{}, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Outcome{}, fmt.Errorf("fetch page returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Outcome{}, fmt.Errorf("parse html: %w", err)
	}

	pageTitle := compactWhitespace(doc.Find("title").First().Text())
	pageText := compactWhitespace(doc.Text())
	if !strings.Contains(strings.ToLower(pageText), strings.ToLower(availabilityMarker)) {
		return Outcome{Available: false, Status: "availability marker not found"}, nil
	}

	extracted, rawResponse, err := s.llmClient.ExtractListing(ctx, targetURL, pageTitle, pageText)
	if err != nil {
		return Outcome{Available: true, Status: "availability marker found but extraction failed"}, fmt.Errorf("extract listing with llm: %w", err)
	}

	extractedJSON, err := json.Marshal(extracted)
	if err != nil {
		return Outcome{Available: true, Status: "availability marker found but serialization failed"}, fmt.Errorf("marshal extracted listing: %w", err)
	}

	sourceURL := strings.TrimSpace(extracted.ListingURL)
	if sourceURL == "" {
		sourceURL = targetURL
	}

	headline := strings.TrimSpace(extracted.Title)
	if headline == "" {
		headline = pageTitle
	}

	fingerprint := buildFingerprint(sourceURL, headline, extracted.Summary, string(extractedJSON))

	return Outcome{
		Available: true,
		Status:    "availability marker found",
		Candidate: &Candidate{
			SourceURL:        sourceURL,
			PageTitle:        pageTitle,
			AvailabilityText: availabilityMarker,
			Headline:         headline,
			Summary:          strings.TrimSpace(extracted.Summary),
			ExtractedData:    datatypes.JSON(extractedJSON),
			LLMRawResponse:   rawResponse,
			Fingerprint:      fingerprint,
		},
	}, nil
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func buildFingerprint(values ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(values, "|")))
	return hex.EncodeToString(hash[:])
}
