package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
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
	defer func() {
		_ = logger.Sync()
	}()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize reporter
	var r *reporter.Reporter
	if !cfg.Agent.Standalone {
		r = reporter.NewReporter(cfg, logger)
		if err := r.Start(ctx); err != nil {
			logger.Fatal("Failed to start reporter", zap.Error(err))
		}
		defer func(r *reporter.Reporter) {
			_ = r.Stop()
		}(r)
	}

	// Initialize notifier
	var n *notify.Manager
	if cfg.Agent.Standalone && cfg.Notify.Enabled {
		n, err = notify.NewManager(cfg.Notify, logger)
		if err != nil {
			logger.Error("Failed to initialize notifier", zap.Error(err))
		} else {
			defer func(n *notify.Manager) {
				_ = n.Stop()
			}(n)
		}
	}

	// Initialize collector manager
	cm := collector.NewManager(cfg, r, n, logger)

	// Initialize handler
	h := handler.NewHandler(cfg, logger, cm)

	// Start components
	components := []struct {
		name  string
		start func(context.Context) error
		stop  func() error
	}{
		{"collector", cm.Start, cm.Stop},
		{"handler", h.Start, h.Stop},
	}

	for _, c := range components {
		if err := c.start(ctx); err != nil {
			logger.Fatal("Failed to start component",
				zap.String("component", c.name),
				zap.Error(err))
		}
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	<-sigChan

	// Graceful shutdown
	cancel()

	// Stop components in reverse order
	for i := len(components) - 1; i >= 0; i-- {
		c := components[i]
		if err := c.stop(); err != nil {
			logger.Error("Failed to stop component",
				zap.String("component", c.name),
				zap.Error(err))
		}
	}

	logger.Info("Shutdown complete")
}
