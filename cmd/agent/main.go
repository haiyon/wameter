package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"wameter/internal/agent/collector"
	"wameter/internal/agent/config"
	"wameter/internal/agent/handler"
	"wameter/internal/agent/notify"
	"wameter/internal/agent/reporter"
	"wameter/internal/logger"
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

	// Run agent
	if err := run(ctx, cfg, logger); err != nil {
		logger.Fatal("Failed to run agent", zap.Error(err))
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

// run runs the agent
func run(ctx context.Context, cfg *config.Config, logger *zap.Logger) (err error) {
	// Initialize reporter
	var r *reporter.Reporter
	if !cfg.Agent.Standalone {
		r = reporter.NewReporter(cfg, logger)
	}

	// Initialize notifier
	var n *notify.Manager
	if cfg.Agent.Standalone && cfg.Notify.Enabled {
		if n, err = notify.NewManager(cfg.Notify, logger); err != nil {
			return fmt.Errorf("failed to initialize notifier: %w", err)
		}
	}

	// Initialize collector and handler
	cm := collector.NewManager(cfg, r, n, logger)
	h := handler.NewHandler(cfg, logger, cm)

	// Start components
	if err = h.Start(ctx); err != nil {
		return fmt.Errorf("failed to start handler: %w", err)
	}

	if err = cm.Start(ctx); err != nil {
		return fmt.Errorf("failed to start collector: %w", err)
	}

	if r != nil {
		if err = r.Start(ctx); err != nil {
			return fmt.Errorf("failed to start reporter: %w", err)
		}
	}

	// Handle cleanup in separate goroutine
	go func() {
		<-ctx.Done()
		// Stop components in reverse order
		if r != nil {
			_ = r.Stop()
		}
		_ = cm.Stop()
		_ = h.Stop()
		if n != nil {
			_ = n.Stop()
		}
	}()

	return nil
}
