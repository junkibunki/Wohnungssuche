package models

import (
	"time"

	"gorm.io/datatypes"
)

type ScrapeTarget struct {
	ID            uint   `gorm:"primaryKey"`
	Name          string `gorm:"size:255"`
	URL           string `gorm:"size:2048;uniqueIndex;not null"`
	Active        bool   `gorm:"not null;default:true"`
	LastCheckedAt *time.Time
	LastStatus    string `gorm:"size:255"`
	LastError     string `gorm:"type:text"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Listings      []Listing
}

type Listing struct {
	ID               uint `gorm:"primaryKey"`
	TargetID         uint `gorm:"index;not null"`
	Target           ScrapeTarget
	SourceURL        string         `gorm:"size:2048;not null"`
	PageTitle        string         `gorm:"size:1024"`
	AvailabilityText string         `gorm:"size:255"`
	Headline         string         `gorm:"size:1024"`
	Summary          string         `gorm:"type:text"`
	ExtractedData    datatypes.JSON `gorm:"type:jsonb;not null"`
	LLMRawResponse   string         `gorm:"type:text"`
	Fingerprint      string         `gorm:"size:64;uniqueIndex;not null"`
	SeenAt           time.Time      `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
