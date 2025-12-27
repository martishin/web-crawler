package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/martishin/web-crawler/internal/config"
	"github.com/martishin/web-crawler/internal/crawler"
	"github.com/martishin/web-crawler/internal/db"
	"github.com/martishin/web-crawler/internal/handler"
	"github.com/martishin/web-crawler/internal/repository"
	"github.com/martishin/web-crawler/internal/route"
	"github.com/martishin/web-crawler/internal/service"
)

var crawlInterval time.Duration

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	rootCmd := &cobra.Command{
		Use:   "api",
		Short: "Habr crawler API server",
		Long:  "HTTP API server with optional background crawling",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cmd.Context(), logger, crawlInterval)
		},
	}

	rootCmd.Flags().DurationVar(
		&crawlInterval,
		"crawl-interval",
		0,
		"run crawler in background at this interval (e.g. 10m, 1h); 0 disables",
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1st Ctrl+C => graceful shutdown (cancel)
	// 2nd Ctrl+C => force exit
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		sig := <-sigCh
		logger.Info("signal received, starting graceful shutdown (Ctrl+C again to force exit)", slog.Any("signal", sig))
		cancel()

		sig = <-sigCh
		logger.Info("second signal received, forcing exit", slog.Any("signal", sig))
		os.Exit(2)
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		logger.Error("command failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func runServer(ctx context.Context, logger *slog.Logger, crawlInterval time.Duration) error {
	// --- Config ---
	srvCfg, err := config.ReadServerConfig()
	if err != nil {
		return err
	}
	dbCfg, err := config.ReadDBConfig()
	if err != nil {
		return err
	}
	crawlerCfg, err := config.ReadCrawlerConfig()
	if err != nil {
		return err
	}

	// --- DB ---
	pool, err := db.NewPool(ctx, dbCfg.DSN)
	if err != nil {
		return err
	}

	if err := db.RunMigrations(pool); err != nil {
		closePoolWithTimeout(pool, logger, 2*time.Second)
		return err
	}

	// --- Wiring ---
	repo := repository.NewRepository(pool)
	cr, err := crawler.New(crawlerCfg, repo)
	if err != nil {
		closePoolWithTimeout(pool, logger, 2*time.Second)
		return err
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

	// Background crawler (interval > 0 => run once on startup + tick)
	var crawlerDone chan struct{}
	if crawlInterval > 0 {
		crawlerDone = make(chan struct{})
		go func() {
			defer close(crawlerDone)
			crawlLoop(ctx, logger, cr, crawlInterval)
		}()
		logger.Info("background crawl enabled", slog.String("interval", crawlInterval.String()))
	}

	// Run HTTP server
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", slog.String("addr", httpServer.Addr))
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		closePoolWithTimeout(pool, logger, 2*time.Second)
		if err != nil {
			return err
		}
		logger.Info("server stopped")
		return nil
	}

	// Graceful shutdown
	logger.Info("context canceled, shutting down http server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("graceful http shutdown failed; forcing close", slog.Any("error", err))
		_ = httpServer.Close()
	}

	if err := <-serverErr; err != nil {
		closePoolWithTimeout(pool, logger, 2*time.Second)
		return err
	}

	if crawlerDone != nil {
		select {
		case <-crawlerDone:
		case <-time.After(2 * time.Second):
			logger.Info("background crawler still stopping (press Ctrl+C again to force exit)")
		}
	}

	closePoolWithTimeout(pool, logger, 2*time.Second)

	logger.Info("server stopped")
	return nil
}

func closePoolWithTimeout(pool *pgxpool.Pool, logger *slog.Logger, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		pool.Close() // may block; we cap it with timeout
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		logger.Info("db pool close timed out; continuing shutdown")
		return
	}
}

func crawlLoop(ctx context.Context, logger *slog.Logger, cr *crawler.Crawler, interval time.Duration) {
	var running int32

	run := func(trigger string) {
		if !atomic.CompareAndSwapInt32(&running, 0, 1) {
			logger.Info("skip crawl: already running", slog.String("trigger", trigger))
			return
		}
		defer atomic.StoreInt32(&running, 0)

		if ctx.Err() != nil {
			return
		}

		logger.Info("crawl started", slog.String("trigger", trigger))

		start := time.Now()
		res, err := cr.UpdateToday(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Info("crawl canceled", slog.String("trigger", trigger))
				return
			}
			logger.Error("crawl failed", slog.Any("error", err), slog.String("trigger", trigger))
			return
		}

		logger.Info("crawl finished",
			slog.Any("result", res),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("trigger", trigger),
		)
	}

	run("startup")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run("ticker")
		}
	}
}
