package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"wameter/internal/database"
	"wameter/internal/logger"
	"wameter/internal/server/api"
	"wameter/internal/server/config"
	"wameter/internal/server/service"
	"wameter/internal/version"

	"go.uber.org/zap"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version if requested
	if *showVersion {
		info := version.GetInfo()
		fmt.Println(info.String())
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logger.New(cfg.Log)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Fatal("Failed to run server", zap.Error(err))
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	<-sigChan

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel()
	<-shutdownCtx.Done()

	logger.Info("Shutdown complete")
}

// run runs the server
func run(ctx context.Context, cfg *config.Config, logger *zap.Logger) error {
	// Initialize database
	db, err := database.New(&cfg.Database, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	defer func(db database.Interface) {
		_ = db.Close()
	}(db)

	// Initialize service
	svc, err := service.NewService(cfg, db, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Create http server
	router := api.NewRouter(cfg, svc, logger)
	server := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: router.Handler(),
	}

	// Start server in background
	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.Background()); err != nil {
			logger.Error("Server shutdown error", zap.Error(err))
		}
	}()

	logger.Info("Starting server", zap.String("address", cfg.Server.Address))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal("Server error", zap.Error(err))
	}

	return nil
}
