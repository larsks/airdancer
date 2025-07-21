package httpserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// StartWithGracefulShutdown starts an HTTP server with graceful shutdown handling
func StartWithGracefulShutdown(addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		log.Printf("starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down server...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Println("server gracefully stopped")
	return nil
}

// Config represents common HTTP server configuration
type Config interface {
	GetListenAddress() string
	GetListenPort() int
}

// StartFromConfig starts an HTTP server using a Config interface
func StartFromConfig(cfg Config, handler http.Handler) error {
	addr := fmt.Sprintf("%s:%d", cfg.GetListenAddress(), cfg.GetListenPort())
	return StartWithGracefulShutdown(addr, handler)
}
