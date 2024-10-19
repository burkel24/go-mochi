package mochi

import (
	"log/slog"
	"net/http"

	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"net"
	"os"
)

func NewRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.DefaultLogger)
	router.Use(middleware.AllowContentType("application/json"))
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("okay xD"))
	})

	return router
}

func NewServer(lc fx.Lifecycle, router *chi.Mux, logger LoggerService) *http.Server {
	portStr := os.Getenv("PORT")
	port := fmt.Sprintf(":%s", portStr)

	srv := &http.Server{Addr: port, Handler: router}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}

			logger.Info("Starting HTTP server", "port", srv.Addr)
			go srv.Serve(ln)

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down HTTP server")

			return srv.Shutdown(ctx)
		},
	})

	return srv
}

func NewFxLogger(logger LoggerService) fxevent.Logger {
	fxLogger := fxevent.SlogLogger{Logger: logger.Logger()}

	fxLogger.UseLogLevel(slog.LevelDebug)
	fxLogger.UseErrorLevel(slog.LevelError)

	return &fxLogger
}

func BuildServerOpts() []fx.Option {
	return []fx.Option{
		fx.Provide(NewRouter),
		fx.Provide(NewServer),
		fx.Invoke(func(*http.Server) {}),
		fx.Provide(NewAuthService),
	}
}

func BuildAppOpts() []fx.Option {
	return []fx.Option{
		fx.WithLogger(NewFxLogger),
		fx.Provide(NewLoggerService),
	}
}
