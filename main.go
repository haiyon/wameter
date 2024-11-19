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

	"github.com/haiyon/wameter/config"
	"github.com/haiyon/wameter/monitor"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	versionFlag := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version information
	if *versionFlag {
		fmt.Printf("Version: %s\nGitCommit: %s\nBuildDate: %s\n", config.Version, config.GitCommit, config.BuildDate)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := setupLogger(cfg, *debug)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Override debug setting if specified
	if *debug {
		cfg.Debug = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

// setupLogger creates a logger with file and console output
func setupLogger(cfg *config.Config, debug bool) (*zap.Logger, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(cfg.LogConfig.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Configure log rotation
	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.LogConfig.Directory, cfg.LogConfig.Filename),
		MaxSize:    cfg.LogConfig.MaxSize,    // MB
		MaxBackups: cfg.LogConfig.MaxBackups, // files
		MaxAge:     cfg.LogConfig.MaxAge,     // days
		Compress:   cfg.LogConfig.Compress,
		LocalTime:  cfg.LogConfig.UseLocalTime,
	}

	// Set log level
	var level zapcore.Level
	switch cfg.LogConfig.LogLevel {
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

	// Force debug level if debug flag is set
	if debug {
		level = zapcore.DebugLevel
	}

	// Configure encoders
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	// Use custom time format if specified
	if cfg.LogConfig.TimeFormat != "" {
		timeFormat := cfg.LogConfig.TimeFormat
		encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(timeFormat))
		}
	}

	// Create console encoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create JSON encoder for file
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create core with both console and file output
	core := zapcore.NewTee(
		// Console output
		zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			level,
		),
		// File output
		zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(logFile),
			level,
		),
	)

	// Create logger
	logger := zap.New(core,
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return logger, nil
}
