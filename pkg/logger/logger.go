package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type FileRotateOptions struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

func New(level, encoding string, outputPaths []string, fileOpts FileRotateOptions) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(lvl),
		Development:      false,
		Encoding:         encoding,
		EncoderConfig:    encoderConfig(),
		OutputPaths:      outputPaths,
		ErrorOutputPaths: []string{"stderr"},
	}

	var buildOpts []zap.Option
	buildOpts = append(buildOpts, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	if fileOpts.Path != "" {
		fileCore, err := buildFileCore(encoding, lvl, fileOpts)
		if err != nil {
			return nil, err
		}
		buildOpts = append(buildOpts, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewTee(core, fileCore)
		}))
	}

	logger, err := cfg.Build(buildOpts...)
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return logger, nil
}

func encoderConfig() zapcore.EncoderConfig {
	c := zap.NewProductionEncoderConfig()
	c.TimeKey = "ts"
	c.EncodeTime = zapcore.ISO8601TimeEncoder
	c.EncodeLevel = zapcore.CapitalLevelEncoder
	c.EncodeDuration = zapcore.StringDurationEncoder
	return c
}

func buildFileCore(encoding string, lvl zapcore.Level, opts FileRotateOptions) (zapcore.Core, error) {
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	rotator := &lumberjack.Logger{
		Filename:   opts.Path,
		MaxSize:    opts.MaxSizeMB,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAgeDays,
		Compress:   opts.Compress,
	}

	encCfg := encoderConfig()
	var enc zapcore.Encoder
	if encoding == "console" {
		enc = zapcore.NewConsoleEncoder(encCfg)
	} else {
		enc = zapcore.NewJSONEncoder(encCfg)
	}
	return zapcore.NewCore(enc, zapcore.AddSync(rotator), lvl), nil
}
