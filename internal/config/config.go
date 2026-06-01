package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL           string
	TelegramBotToken      string
	AllowedTelegramChatID int64
	LLMAPIURL             string
	LLMAPIKey             string
	LLMModel              string
	ScrapeInterval        time.Duration
	HTTPTimeout           time.Duration
	MaxPageTextChars      int
	UserAgent             string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		LLMAPIURL:        defaultString("LLM_API_URL", "https://api.openai.com/v1/chat/completions"),
		LLMAPIKey:        strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		LLMModel:         defaultString("LLM_MODEL", "gpt-4.1-mini"),
		ScrapeInterval:   defaultDuration("SCRAPE_INTERVAL", 15*time.Minute),
		HTTPTimeout:      defaultDuration("HTTP_TIMEOUT", 30*time.Second),
		MaxPageTextChars: defaultInt("MAX_PAGE_TEXT_CHARS", 15000),
		UserAgent:        defaultString("USER_AGENT", "WohnungssucheBot/1.0 (+https://telegram.org)"),
	}

	allowedChatID := strings.TrimSpace(os.Getenv("TELEGRAM_ALLOWED_CHAT_ID"))
	if allowedChatID != "" {
		parsedChatID, err := strconv.ParseInt(allowedChatID, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse TELEGRAM_ALLOWED_CHAT_ID: %w", err)
		}
		cfg.AllowedTelegramChatID = parsedChatID
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	if cfg.LLMAPIURL == "" {
		return Config{}, fmt.Errorf("LLM_API_URL is required")
	}

	if cfg.LLMModel == "" {
		return Config{}, fmt.Errorf("LLM_MODEL is required")
	}

	if cfg.ScrapeInterval <= 0 {
		return Config{}, fmt.Errorf("SCRAPE_INTERVAL must be greater than 0")
	}

	if cfg.HTTPTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP_TIMEOUT must be greater than 0")
	}

	if cfg.MaxPageTextChars < 1000 {
		return Config{}, fmt.Errorf("MAX_PAGE_TEXT_CHARS must be at least 1000")
	}

	return cfg, nil
}

func defaultString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func defaultDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func defaultInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
