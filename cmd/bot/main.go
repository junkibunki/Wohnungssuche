package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	"wohnungssuche/internal/bot"
	"wohnungssuche/internal/config"
	"wohnungssuche/internal/database"
	"wohnungssuche/internal/llm"
	"wohnungssuche/internal/scraper"
	"wohnungssuche/internal/service"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	llmClient := llm.New(cfg)
	scraperClient := scraper.New(cfg, llmClient)
	appService := service.New(db, scraperClient)

	telegramAPI, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("create telegram bot: %v", err)
	}

	telegramBot := bot.New(telegramAPI, appService, cfg.AllowedTelegramChatID)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go runScheduler(ctx, cfg.ScrapeInterval, appService, telegramBot)

	if err := telegramBot.Run(ctx); err != nil {
		log.Fatalf("run telegram bot: %v", err)
	}
}

func runScheduler(ctx context.Context, interval time.Duration, appService *service.Service, telegramBot *bot.Bot) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report, err := appService.ScrapeAll(ctx)
			if err != nil {
				log.Printf("scheduled scrape failed: %v", err)
				continue
			}

			if err := telegramBot.NotifyNewListings(report.NewListings); err != nil {
				log.Printf("send listing notification failed: %v", err)
			}
		}
	}
}
