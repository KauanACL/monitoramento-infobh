package server

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func RunCommand() {
	cfg := Config{
		Addr:          envAddr(),
		DBPath:        envString("MONITOR_DB_PATH", DefaultDBPath),
		RetentionDays: envInt("MONITOR_RETENTION_DAYS", DefaultRetentionDays),
		ActionPIN:     envString("MONITOR_ACTION_PIN", DefaultActionPIN),
	}

	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "endereco HTTP do dashboard e API")
	flag.StringVar(&cfg.DBPath, "db", cfg.DBPath, "caminho do banco SQLite")
	flag.IntVar(&cfg.RetentionDays, "retention-days", cfg.RetentionDays, "dias de retencao das metricas detalhadas")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	store, err := OpenStore(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	app := NewApp(store, cfg, logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app.StartRetentionJob(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("monitoramento server started", "addr", cfg.Addr, "db", cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped with error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envAddr() string {
	if value := os.Getenv("MONITOR_ADDR"); value != "" {
		return value
	}
	if value := os.Getenv("PORT"); value != "" {
		return ":" + value
	}
	return DefaultAddr
}

func envInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
