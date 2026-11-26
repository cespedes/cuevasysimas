// Package config reads the configuration of the server from the environment.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string

	// PublicSiteURL is the address of the public site (https://cuevasysimas.es): the host it answers to, and the
	// base of the addresses in its pages and its sitemap. Empty: the site is off. PublicSiteLegal* are who
	// answers for it (name, address and email), which its legal notice and its privacy policy say; until all
	// three are given the pages say they are pending and ask search engines to stay out.
	PublicSiteURL          string
	PublicSiteLegalName    string
	PublicSiteLegalAddress string
	PublicSiteLegalEmail   string

	// PublicRateLimit/PublicRateWindow throttle the public routes and pages, which answer people who are not
	// signed in, per calling address: how many requests in how long. Not positive: no limit.
	PublicRateLimit  int
	PublicRateWindow time.Duration

	// PlaceFilesRoot is the directory that the photos and documents of the places are written to.
	PlaceFilesRoot string
	// NominatimURL is the instance that proposes the zone of a coordinate ("off" turns it off).
	NominatimURL string

	// TelegramBotToken and TelegramLogChatID are the project's own bot and channel, which hear of what changes
	// in the catalogue and of errors. Both empty: nothing is sent.
	TelegramBotToken  string
	TelegramLogChatID string
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:               getEnv("HTTP_ADDR", ":64711"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		PublicSiteURL:          os.Getenv("PUBLIC_SITE_URL"),
		PublicSiteLegalName:    os.Getenv("PUBLIC_SITE_LEGAL_NAME"),
		PublicSiteLegalAddress: os.Getenv("PUBLIC_SITE_LEGAL_ADDRESS"),
		PublicSiteLegalEmail:   os.Getenv("PUBLIC_SITE_LEGAL_EMAIL"),
		PublicRateLimit:        120,
		PublicRateWindow:       time.Minute,
		PlaceFilesRoot:         getEnv("PLACE_FILES_ROOT", "/data/place-files"),
		NominatimURL:           getEnv("NOMINATIM_URL", "https://nominatim.openstreetmap.org/reverse"),
		TelegramBotToken:       os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramLogChatID:      os.Getenv("TELEGRAM_LOG_CHAT_ID"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	cfg.PlaceFilesRoot = filepath.Clean(cfg.PlaceFilesRoot)
	return cfg, nil
}
