package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"wameter/internal/server/api"
	"wameter/internal/server/config"
	"wameter/internal/server/database"
	"wameter/internal/server/service"
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
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database
	db, err := database.NewDatabase(&cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	defer func(db database.Database) {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database", zap.Error(err))
		}
	}(db)

	// Initialize service
	svc, err := service.NewService(cfg, db, logger)
	if err != nil {
		logger.Fatal("Failed to initialize service", zap.Error(err))
	}
	defer func(svc *service.Service) {
		if err := svc.Stop(); err != nil {
			logger.Error("Failed to stop service", zap.Error(err))
		}
	}(svc)

	// Initialize router
	router := api.NewRouter(cfg, svc, logger)

	// Create HTTP server
	server := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: router.Handler(),
	}

	// Start server in background
	go func() {
		logger.Info("Starting server",
			zap.String("address", cfg.Server.Address))
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Server error", zap.Error(err))
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
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
	w := &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

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

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	// Create core with both file and console output
	core := zapcore.NewTee(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(w),
			level,
		),
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		),
	)

	// Create logger
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return logger.Named("server"), nil
}
