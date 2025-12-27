package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/martishin/web-crawler/internal/config"
	"github.com/martishin/web-crawler/internal/crawler"
	"github.com/martishin/web-crawler/internal/db"
	"github.com/martishin/web-crawler/internal/handler"
	"github.com/martishin/web-crawler/internal/repository"
	"github.com/martishin/web-crawler/internal/route"
	"github.com/martishin/web-crawler/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// --- Config ---
	srvCfg, err := config.ReadServerConfig()
	if err != nil {
		logger.Error("read server config", slog.Any("error", err))
		os.Exit(1)
	}
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

	// --- DB ---
	ctx := context.Background()
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

	// --- Wiring ---
	repo := repository.NewRepository(pool)

	cr, err := crawler.New(crawlerCfg, repo)
	if err != nil {
		logger.Error("init crawler", slog.Any("error", err))
		os.Exit(1)
	}

	svc := service.New(repo, cr)
	api := handler.NewAPI(svc)

	h := route.Routes(logger, api)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", srvCfg.Port),
		Handler:      h,
		ReadTimeout:  srvCfg.ReadTimeout,
		WriteTimeout: srvCfg.WriteTimeout,
		IdleTimeout:  srvCfg.IdleTimeout,
	}

	// --- Run server in foreground ---
	logger.Info("starting server", slog.String("addr", httpServer.Addr))

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("server stopped", slog.Time("time", time.Now()))
}
