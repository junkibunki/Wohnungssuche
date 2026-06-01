package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"wohnungssuche/internal/models"
	"wohnungssuche/internal/scraper"
)

type ScrapeReport struct {
	CheckedTargets int
	NewListings    []models.Listing
	Errors         []string
}

type Service struct {
	db      *gorm.DB
	scraper *scraper.Scraper
}

func New(db *gorm.DB, scraperClient *scraper.Scraper) *Service {
	return &Service{db: db, scraper: scraperClient}
}

func (s *Service) AddTarget(ctx context.Context, rawURL string, name string) (models.ScrapeTarget, error) {
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return models.ScrapeTarget{}, fmt.Errorf("invalid url: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return models.ScrapeTarget{}, fmt.Errorf("url must use http or https")
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		trimmedName = parsedURL.Host
	}

	var target models.ScrapeTarget
	err = s.db.WithContext(ctx).Where("url = ?", parsedURL.String()).First(&target).Error
	if err == nil {
		target.Name = trimmedName
		target.Active = true
		if err := s.db.WithContext(ctx).Save(&target).Error; err != nil {
			return models.ScrapeTarget{}, fmt.Errorf("update scrape target: %w", err)
		}
		return target, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ScrapeTarget{}, fmt.Errorf("lookup scrape target: %w", err)
	}

	target = models.ScrapeTarget{
		Name:   trimmedName,
		URL:    parsedURL.String(),
		Active: true,
	}
	if err := s.db.WithContext(ctx).Create(&target).Error; err != nil {
		return models.ScrapeTarget{}, fmt.Errorf("create scrape target: %w", err)
	}

	return target, nil
}

func (s *Service) ListTargets(ctx context.Context) ([]models.ScrapeTarget, error) {
	var targets []models.ScrapeTarget
	if err := s.db.WithContext(ctx).Order("id asc").Find(&targets).Error; err != nil {
		return nil, fmt.Errorf("list scrape targets: %w", err)
	}

	return targets, nil
}

func (s *Service) RemoveTarget(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.ScrapeTarget{}, id)
	if result.Error != nil {
		return fmt.Errorf("remove scrape target: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("scrape target %d not found", id)
	}

	return nil
}

func (s *Service) ScrapeAll(ctx context.Context) (ScrapeReport, error) {
	var targets []models.ScrapeTarget
	if err := s.db.WithContext(ctx).Where("active = ?", true).Order("id asc").Find(&targets).Error; err != nil {
		return ScrapeReport{}, fmt.Errorf("load scrape targets: %w", err)
	}

	report := ScrapeReport{CheckedTargets: len(targets)}
	for _, target := range targets {
		now := time.Now().UTC()
		outcome, err := s.scraper.Scrape(ctx, target.URL)
		status := outcome.Status
		lastError := ""
		if status == "" {
			status = "scrape failed"
		}
		if err != nil {
			lastError = err.Error()
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", target.URL, err))
		}

		if updateErr := s.db.WithContext(ctx).Model(&models.ScrapeTarget{}).Where("id = ?", target.ID).Updates(map[string]any{
			"last_checked_at": now,
			"last_status":     status,
			"last_error":      lastError,
		}).Error; updateErr != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: update status failed: %v", target.URL, updateErr))
			continue
		}

		if err != nil || outcome.Candidate == nil {
			continue
		}

		listing := models.Listing{
			TargetID:         target.ID,
			SourceURL:        outcome.Candidate.SourceURL,
			PageTitle:        outcome.Candidate.PageTitle,
			AvailabilityText: outcome.Candidate.AvailabilityText,
			Headline:         outcome.Candidate.Headline,
			Summary:          outcome.Candidate.Summary,
			ExtractedData:    outcome.Candidate.ExtractedData,
			LLMRawResponse:   outcome.Candidate.LLMRawResponse,
			Fingerprint:      outcome.Candidate.Fingerprint,
			SeenAt:           now,
		}

		created, createErr := s.createListingIfNew(ctx, listing)
		if createErr != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: store listing failed: %v", target.URL, createErr))
			continue
		}
		if created {
			report.NewListings = append(report.NewListings, listing)
		}
	}

	sort.Slice(report.NewListings, func(i int, j int) bool {
		return report.NewListings[i].ID < report.NewListings[j].ID
	})

	return report, nil
}

func (s *Service) createListingIfNew(ctx context.Context, listing models.Listing) (bool, error) {
	var existing models.Listing
	err := s.db.WithContext(ctx).Where("fingerprint = ?", listing.Fingerprint).First(&existing).Error
	if err == nil {
		return false, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("lookup listing fingerprint: %w", err)
	}

	if err := s.db.WithContext(ctx).Create(&listing).Error; err != nil {
		return false, fmt.Errorf("create listing: %w", err)
	}

	return true, nil
}
