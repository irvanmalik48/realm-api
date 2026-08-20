package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/router"
)

func main() {
	cfg := config.Load()

	// Initialize Database connection if configured
	var db *database.DB
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		db, err = database.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v\n", err)
		} else {
			defer db.Close()
		}
	}

	app := router.New(cfg, db)

	// Channel for idle connections / graceful shutdown
	idleConnsClosed := make(chan struct{})

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Received shutdown signal, gracefully shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Printf("Server shutdown error: %v\n", err)
		}
		if db != nil {
			db.Close()
		}
		close(idleConnsClosed)
	}()

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Realm API starting on %s (%s mode)\n", addr, cfg.Environment)
	if err := app.Listen(addr); err != nil {
		log.Printf("Server exited with error: %v\n", err)
	}

	<-idleConnsClosed
	log.Println("Server stopped.")
}
