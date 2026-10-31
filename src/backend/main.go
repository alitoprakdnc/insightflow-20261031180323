package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/insightflow/src/backend/config"
	"github.com/yourorg/insightflow/src/backend/db"
	"github.com/yourorg/insightflow/src/backend/router"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger (using standard log for simplicity)
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)

	// Set up PostgreSQL connection pool
	pg, err := db.NewPostgresDB(cfg)
	if err != nil {
		logger.Fatalf("failed to initialize database: %v", err)
	}
	defer func() {
		if cerr := pg.Close(); cerr != nil {
			logger.Printf("error closing database connection: %v", cerr)
		}
	}()

	// Run database migrations
	if err := pg.Migrate(); err != nil {
		logger.Fatalf("database migration failed: %v", err)
	}

	// Build HTTP router with all routes and middleware
	r := router.NewRouter(pg, cfg)

	// Prepare HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ServerPort),
		Handler: r,
	}

	// Channel to capture server errors
	serverErrChan := make(chan error, 1)

	// Start server in a separate goroutine
	go func() {
		logger.Printf("starting server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
		close(serverErrChan)
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Printf("received signal %s, initiating graceful shutdown", sig)
	case err := <-serverErrChan:
		logger.Fatalf("server error: %v", err)
	}

	// Context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("graceful shutdown failed: %v", err)
	}

	logger.Println("server stopped")
}