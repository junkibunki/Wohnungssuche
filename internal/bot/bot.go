package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"wohnungssuche/internal/models"
	"wohnungssuche/internal/service"
)

type Bot struct {
	api                *tgbotapi.BotAPI
	service            *service.Service
	allowedChatID      int64
	notificationChatID int64
	mu                 sync.RWMutex
}

func New(api *tgbotapi.BotAPI, service *service.Service, allowedChatID int64) *Bot {
	return &Bot{
		api:                api,
		service:            service,
		allowedChatID:      allowedChatID,
		notificationChatID: allowedChatID,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := b.api.GetUpdatesChan(updateConfig)
	defer b.api.StopReceivingUpdates()

	for {
		select {
		case <-ctx.Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message == nil {
				continue
			}

			chatID := update.Message.Chat.ID
			if !b.isAuthorized(chatID) {
				_, _ = b.api.Send(tgbotapi.NewMessage(chatID, "This bot is restricted to a configured Telegram chat."))
				continue
			}

			b.rememberChat(chatID)

			response, err := b.handleMessage(ctx, update.Message)
			if err != nil {
				response = fmt.Sprintf("Error: %v", err)
			}

			if response == "" {
				continue
			}

			msg := tgbotapi.NewMessage(chatID, response)
			msg.ParseMode = "Markdown"
			_, _ = b.api.Send(msg)
		}
	}
}

func (b *Bot) NotifyNewListings(listings []models.Listing) error {
	if len(listings) == 0 {
		return nil
	}

	chatID := b.notificationChat()
	if chatID == 0 {
		return nil
	}

	for _, listing := range listings {
		message := formatListing(listing)
		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = "Markdown"
		if _, err := b.api.Send(msg); err != nil {
			return fmt.Errorf("send telegram message: %w", err)
		}
	}

	return nil
}

func (b *Bot) handleMessage(ctx context.Context, message *tgbotapi.Message) (string, error) {
	switch message.Command() {
	case "start", "help":
		return helpText(), nil
	case "addurl":
		urlValue, name, err := parseAddURLArguments(message.CommandArguments())
		if err != nil {
			return "", err
		}

		target, err := b.service.AddTarget(ctx, urlValue, name)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Tracking target %d\n%s", target.ID, target.URL), nil
	case "listurls":
		targets, err := b.service.ListTargets(ctx)
		if err != nil {
			return "", err
		}

		if len(targets) == 0 {
			return "No URLs configured yet.", nil
		}

		lines := make([]string, 0, len(targets))
		for _, target := range targets {
			status := target.LastStatus
			if status == "" {
				status = "never checked"
			}
			lines = append(lines, fmt.Sprintf("%d. %s\n%s\nStatus: %s", target.ID, target.Name, target.URL, status))
		}

		return strings.Join(lines, "\n\n"), nil
	case "removeurl":
		idValue, err := strconv.ParseUint(strings.TrimSpace(message.CommandArguments()), 10, 64)
		if err != nil {
			return "", fmt.Errorf("usage: /removeurl <id>")
		}

		if err := b.service.RemoveTarget(ctx, uint(idValue)); err != nil {
			return "", err
		}

		return fmt.Sprintf("Removed target %d", idValue), nil
	case "scrape":
		report, err := b.service.ScrapeAll(ctx)
		if err != nil {
			return "", err
		}

		if notifyErr := b.NotifyNewListings(report.NewListings); notifyErr != nil {
			return "", notifyErr
		}

		return formatScrapeReport(report), nil
	default:
		return helpText(), nil
	}
}

func (b *Bot) isAuthorized(chatID int64) bool {
	return b.allowedChatID == 0 || b.allowedChatID == chatID
}

func (b *Bot) rememberChat(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.allowedChatID == 0 {
		b.notificationChatID = chatID
	}
}

func (b *Bot) notificationChat() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.notificationChatID
}

func parseAddURLArguments(arguments string) (string, string, error) {
	fields := strings.Fields(strings.TrimSpace(arguments))
	if len(fields) == 0 {
		return "", "", fmt.Errorf("usage: /addurl <url> [name]")
	}

	urlValue := fields[0]
	name := strings.Join(fields[1:], " ")
	return urlValue, name, nil
}

func helpText() string {
	return strings.Join([]string{
		"Available commands:",
		"/addurl <url> [name] - add a page to monitor",
		"/listurls - show all configured pages",
		"/removeurl <id> - remove a page",
		"/scrape - trigger scraping immediately",
	}, "\n")
}

func formatScrapeReport(report service.ScrapeReport) string {
	response := fmt.Sprintf("Checked %d targets\nNew listings: %d", report.CheckedTargets, len(report.NewListings))
	if len(report.Errors) > 0 {
		response += fmt.Sprintf("\nErrors: %d", len(report.Errors))
	}
	return response
}

func formatListing(listing models.Listing) string {
	var builder strings.Builder
	builder.WriteString("*Neue Mietwohnung erkannt*\n")
	if listing.Headline != "" {
		builder.WriteString(listing.Headline)
		builder.WriteString("\n")
	}
	if listing.Summary != "" {
		builder.WriteString(listing.Summary)
		builder.WriteString("\n")
	}
	builder.WriteString(listing.SourceURL)
	return builder.String()
}
