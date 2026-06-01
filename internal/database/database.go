package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"wohnungssuche/internal/config"
	"wohnungssuche/internal/models"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := db.AutoMigrate(&models.ScrapeTarget{}, &models.Listing{}); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}
