package mochi

import (
	"log/slog"
	"os"

	"go.uber.org/fx"
)

type LoggerService interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Logger() *slog.Logger
}

type LoggerServiceParams struct {
	fx.In
}

type LoggerServiceResult struct {
	fx.Out

	LoggerService LoggerService
}

type loggerService struct {
	logger *slog.Logger
}

func NewLoggerService(params LoggerServiceParams) (LoggerServiceResult, error) {
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	srv := &loggerService{logger: logger}
	return LoggerServiceResult{LoggerService: srv}, nil
}

func (srv *loggerService) Debug(msg string, args ...any) {
	srv.logger.Debug(msg, args...)
}

func (srv *loggerService) Info(msg string, args ...any) {
	srv.logger.Info(msg, args...)
}

func (srv *loggerService) Warn(msg string, args ...any) {
	srv.logger.Warn(msg, args...)
}

func (srv *loggerService) Error(msg string, args ...any) {
	srv.logger.Error(msg, args...)
}

func (srv *loggerService) Logger() *slog.Logger {
	return srv.logger
}
