// Command api serves the invoice flow.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pasipiya/invoice-service-quest/internal/httpapi"
	"github.com/pasipiya/invoice-service-quest/internal/invoice"
	"github.com/pasipiya/invoice-service-quest/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, env("DATABASE_URL", defaultDSN))
	if err != nil {
		return err
	}
	defer st.Close()

	addr := env("HTTP_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(invoice.New(st)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// defaultDSN points at the local docker-compose database. Local throwaway
// credentials only; see .env.example.
const defaultDSN = "postgres://postgres:postgres@localhost:5433/invoices?sslmode=disable"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
