package main

import (
	"context"
	"errors"
	"feitu/backend/internal/dependencies"
	"feitu/backend/internal/health"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	checks, closeClients := dependencies.New()
	defer closeClients()
	server := &http.Server{Addr: dependencies.Env("HTTP_ADDR", "127.0.0.1:8080"), Handler: (health.Handler{Checks: checks, Timeout: dependencies.Timeout}).Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { log.Print("Backend HTTP server starting"); errCh <- server.ListenAndServe() }()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Print("HTTP server failed to start or stopped unexpectedly")
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
		log.Print("Backend stopped")
	}
}
