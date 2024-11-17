package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/haiyon/ip-monitor/config"
	"github.com/haiyon/ip-monitor/monitor"
	"go.uber.org/zap"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logger
	logConfig := zap.NewProductionConfig()
	if *debug {
		logConfig = zap.NewDevelopmentConfig()
	}
	logger, err := logConfig.Build()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
		}
	}(logger)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// Override debug setting if specified
	if *debug {
		cfg.Debug = true
	}

	// Create and start monitor
	m, err := monitor.NewMonitor(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create monitor", zap.Error(err))
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitor in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- m.Start()
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal", zap.String("signal", sig.String()))
	case err := <-errChan:
		if err != nil {
			logger.Error("Monitor error", zap.Error(err))
		}
	}

	// Graceful shutdown
	logger.Info("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := m.Stop(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
		os.Exit(1)
	}
}
