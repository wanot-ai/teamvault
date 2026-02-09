// TeamVault API Server
//
// Usage:
//
//	server            Start the HTTP server
//	server -migrate   Run database migrations and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teamvault/teamvault/internal/api"
	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/auth"
	"github.com/teamvault/teamvault/internal/crypto"
	"github.com/teamvault/teamvault/internal/db"
	"github.com/teamvault/teamvault/internal/policy"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "Run migrations and exit")
	migrationsDir := flag.String("migrations-dir", "migrations", "Path to migrations directory")
	flag.Parse()

	ctx := context.Background()

	// Load config from environment
	databaseURL := requireEnv("DATABASE_URL")
	jwtSecret := requireEnv("JWT_SECRET")
	listenAddr := getEnv("LISTEN_ADDR", ":8443")

	// Connect to database
	database, err := db.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.RunMigrations(ctx, *migrationsDir); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations complete")

	if *migrateOnly {
		log.Println("Migration-only mode, exiting")
		return
	}

	// Initialize crypto (envelope encryption)
	cryptoSvc, err := crypto.NewEnvelopeCryptoFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize crypto: %v", err)
	}
	log.Println("Envelope encryption initialized")

	// Initialize auth
	authSvc := auth.New(jwtSecret)

	// Initialize policy engine
	policySvc := policy.NewEngine(database)

	// Initialize audit logger
	auditSvc := audit.NewLogger(database)

	// Create API server
	apiServer := api.NewServer(database, authSvc, cryptoSvc, policySvc, auditSvc)

	// Create HTTP server
	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      apiServer.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("TeamVault API server starting on %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		fmt.Fprintf(os.Stderr, "Required environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return val
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
