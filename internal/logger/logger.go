package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// New creates a new logger instance with the provided configuration
func New(cfg *Config) (*zap.Logger, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	cfg = cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logger config: %w", err)
	}

	// Create log directory if file path is specified
	if cfg.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Configure encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Set log level
	level := getZapLevel(cfg.Level)

	var cores []zapcore.Core

	// Add file output if configured
	if cfg.File != "" {
		w := &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}

		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(w),
			level,
		))
	}

	// Add console output
	cores = append(cores, zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	))

	// Create logger with multiple outputs
	core := zapcore.NewTee(cores...)
	return zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	), nil
}

// getZapLevel converts string level to zapcore.Level
func getZapLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
