package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/martishin/web-crawler/internal/config"
	"github.com/martishin/web-crawler/internal/crawler"
	"github.com/martishin/web-crawler/internal/db"
	"github.com/martishin/web-crawler/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	var once bool
	flag.BoolVar(&once, "once", true, "run crawler once")
	flag.Parse()
	_ = once

	ctx := context.Background()

	dbCfg, err := config.ReadDBConfig()
	if err != nil {
		logger.Error("read db config", slog.Any("error", err))
		os.Exit(1)
	}
	crawlerCfg, err := config.ReadCrawlerConfig()
	if err != nil {
		logger.Error("read crawler config", slog.Any("error", err))
		os.Exit(1)
	}

	pool, err := db.NewPool(ctx, dbCfg.DSN)
	if err != nil {
		logger.Error("connect db", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(pool); err != nil {
		logger.Error("run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	repo := repository.NewRepository(pool)

	cr, err := crawler.New(crawlerCfg, repo)
	if err != nil {
		logger.Error("init crawler", slog.Any("error", err))
		os.Exit(1)
	}

	res, err := cr.UpdateToday(ctx)
	if err != nil {
		logger.Error("crawl failed", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("crawl finished", slog.Any("result", res))
}
