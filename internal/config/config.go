package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DBConfig struct {
	DSN string
}

type CrawlerConfig struct {
	BaseURL        string
	IndexPath      string
	UserAgent      string
	MaxConcurrency int
	ReqTimeout     time.Duration
	PerHostQPS     float64
	Timezone       string
	MaxPages       int
	PoliteDelay    time.Duration
}

func ReadServerConfig() (*ServerConfig, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8100"))
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}
	return &ServerConfig{
		Port:         port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, nil
}

func ReadDBConfig() (*DBConfig, error) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("PG_DSN is required")
	}
	return &DBConfig{DSN: dsn}, nil
}

func ReadCrawlerConfig() (*CrawlerConfig, error) {
	maxConc, err := strconv.Atoi(getEnv("CRAWLER_MAX_CONCURRENCY", "8"))
	if err != nil {
		return nil, fmt.Errorf("invalid CRAWLER_MAX_CONCURRENCY: %w", err)
	}
	qps, err := strconv.ParseFloat(getEnv("CRAWLER_PER_HOST_QPS", "2"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid CRAWLER_PER_HOST_QPS: %w", err)
	}
	timeoutSec, err := strconv.Atoi(getEnv("CRAWLER_TIMEOUT_SECONDS", "15"))
	if err != nil {
		return nil, fmt.Errorf("invalid CRAWLER_TIMEOUT_SECONDS: %w", err)
	}
	maxPages, err := strconv.Atoi(getEnv("CRAWLER_MAX_PAGES", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid CRAWLER_MAX_PAGES: %w", err)
	}
	delayMs, err := strconv.Atoi(getEnv("CRAWLER_POLITE_DELAY_MS", "250"))
	if err != nil {
		return nil, fmt.Errorf("invalid CRAWLER_POLITE_DELAY_MS: %w", err)
	}

	return &CrawlerConfig{
		BaseURL:        getEnv("HABR_BASE_URL", "https://habr.com"),
		IndexPath:      getEnv("HABR_INDEX_PATH", "/ru/all/"),
		UserAgent:      getEnv("CRAWLER_USER_AGENT", "habr-crawler-go/1.0"),
		MaxConcurrency: maxConc,
		ReqTimeout:     time.Duration(timeoutSec) * time.Second,
		PerHostQPS:     qps,
		Timezone:       getEnv("CRAWLER_TIMEZONE", "Europe/Moscow"),
		MaxPages:       maxPages,
		PoliteDelay:    time.Duration(delayMs) * time.Millisecond,
	}, nil
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
