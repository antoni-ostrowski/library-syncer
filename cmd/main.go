package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/config"
	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/runner"
	"github.com/antoni-ostrowski/library-syncer/internal/web/handlers"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	cfg := config.RunConfig()
	dbSvc := db.NewDbService(cfg.DB)
	run := runner.New(dbSvc, cfg.SleepSec, cfg.DevMode)
	go run.Start(ctx)
	run.Trigger(runner.Cmd{Type: runner.CmdTypeRunAll})

	mux := http.NewServeMux()
	handlers.Register(mux, dbSvc, run)

	srv := http.Server{
		Addr:              ":3000",
		Handler:           mux,
		ReadHeaderTimeout: time.Second * 5,
	}
	srvErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "address", "http://localhost:3000")
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		// Startup failed: nothing to drain, and the pool never served
		// traffic, so exiting directly is safe.
		stop()
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		// First signal: stop listening for more, so a second Ctrl+C
		// kills immediately. Drain in-flight work within budget.
		stop()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			slog.Error("shutdown error", "err", err)
		}
		slog.Info("stopped")
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalln("server error: ", err)
	}
}
