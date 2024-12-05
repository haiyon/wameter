package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"wameter/internal/agent/collector"
	"wameter/internal/agent/collector/network"
	"wameter/internal/agent/config"
	"wameter/internal/agent/handler"
	"wameter/internal/agent/reporter"
	"wameter/internal/version"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
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
	logger, err := initLogger(cfg.Log)
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

	// Initialize components
	cm := collector.NewManager(cfg, logger)
	r := reporter.NewReporter(cfg, logger)
	h := handler.NewHandler(cfg, logger, cm)

	// Register collectors
	networkCollector := network.NewCollector(&cfg.Collector.Network, cfg.Agent.ID, logger)
	if err := cm.RegisterCollector(networkCollector); err != nil {
		logger.Fatal("Failed to register network collector", zap.Error(err))
	}

	// Start components
	components := []struct {
		name  string
		start func(context.Context) error
		stop  func() error
	}{
		{"reporter", r.Start, r.Stop},
		{"handler", h.Start, h.Stop},
		{"collector", cm.Start, cm.Stop},
	}

	for _, c := range components {
		if err := c.start(ctx); err != nil {
			logger.Fatal("Failed to start component",
				zap.String("component", c.name),
				zap.Error(err))
		}
	}

	// Start collection loop
	go func() {
		ticker := time.NewTicker(cfg.Collector.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				data, err := cm.Collect(ctx)
				if err != nil {
					logger.Error("Failed to collect metrics", zap.Error(err))
					continue
				}

				if err := r.Report(data); err != nil {
					logger.Error("Failed to report metrics", zap.Error(err))
				}
			}
		}
	}()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	logger.Info("Received signal", zap.String("signal", sig.String()))

	// Graceful shutdown
	logger.Info("Starting graceful shutdown")
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

func initLogger(cfg config.LogConfig) (*zap.Logger, error) {
	// Check if the log file path exists
	_, err := os.Stat(cfg.File)
	if os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to check log file path: %w", err)
	}

	// Configure log rotation
	_ = &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// Create logger config
	zapCfg := zap.NewProductionConfig()
	zapCfg.OutputPaths = []string{"stdout", cfg.File}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	// Set log level
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	zapCfg.EncoderConfig = encoderConfig

	// Create logger
	logger := zap.Must(zapCfg.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	))

	return logger.Named("agent"), nil
}
