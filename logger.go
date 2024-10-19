package mochi

import (
	"log/slog"
	"os"

	"github.com/burkel24/go-mochi/internal"
	"go.uber.org/fx"
)

type LoggerServiceParams struct {
	fx.In
}

type LoggerServiceResult struct {
	fx.Out

	LoggerService internal.LoggerService
}

type LoggerService struct {
	logger *slog.Logger
}

func NewLoggerService(params LoggerServiceParams) (LoggerServiceResult, error) {
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	srv := &LoggerService{logger: logger}
	return LoggerServiceResult{LoggerService: srv}, nil
}

func (srv *LoggerService) Debug(msg string, args ...any) {
	srv.logger.Debug(msg, args...)
}

func (srv *LoggerService) Info(msg string, args ...any) {
	srv.logger.Info(msg, args...)
}

func (srv *LoggerService) Warn(msg string, args ...any) {
	srv.logger.Warn(msg, args...)
}

func (srv *LoggerService) Error(msg string, args ...any) {
	srv.logger.Error(msg, args...)
}

func (srv *LoggerService) Logger() *slog.Logger {
	return srv.logger
}
