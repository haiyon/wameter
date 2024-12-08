package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"wameter/internal/agent/collector"
	"wameter/internal/agent/config"
	"wameter/internal/agent/handler"
	"wameter/internal/agent/notify"
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
