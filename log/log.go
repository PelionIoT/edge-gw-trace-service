package log

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type key int

const (
	contextKey key = iota
	loggerKey  key = iota
)

// WithContext enriches the logger with fields from the context
func WithContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	return logger.With(Fields(ctx)...)
}

// WithFields adds log fields to the context
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return context.WithValue(ctx, contextKey, append(Fields(ctx), fields...))
}

// Fields extracts log fields from the context
func Fields(ctx context.Context) []zap.Field {
	rawFields := ctx.Value(contextKey)

	if rawFields == nil {
		return []zap.Field{}
	}

	fields, ok := rawFields.([]zap.Field)

	if !ok {
		return []zap.Field{}
	}

	return fields
}

// ZapLogLevel returns zap log level with specified log level
func ZapLogLevel(logLevel string) zapcore.Level {
	switch strings.ToLower(logLevel) {
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}
