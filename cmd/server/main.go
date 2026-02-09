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
	"github.com/teamvault/teamvault/internal/lease"
	"github.com/teamvault/teamvault/internal/policy"
	"github.com/teamvault/teamvault/internal/rotation"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "Run migrations and exit")
	migrationsDir := flag.String("migrations-dir", "migrations", "Path to migrations directory")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Initialize OIDC client (optional — graceful degradation if not configured)
	oidcConfig := auth.OIDCConfig{
		Issuer:       os.Getenv("OIDC_ISSUER"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("OIDC_REDIRECT_URI"),
	}
	oidcClient := auth.NewOIDCClient(oidcConfig)
	if oidcClient != nil {
		if err := oidcClient.Discover(ctx); err != nil {
			log.Printf("WARNING: OIDC discovery failed (OIDC login will be unavailable): %v", err)
			oidcClient = nil
		} else {
			log.Printf("OIDC configured with issuer: %s", oidcConfig.Issuer)
		}
	} else {
		log.Println("OIDC not configured (set OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URI to enable)")
	}

	// Initialize rotation scheduler
	rotationScheduler := rotation.NewScheduler(database, cryptoSvc)
	go rotationScheduler.Start(ctx)
	log.Println("Rotation scheduler started")

	// Initialize lease manager
	leaseManager := lease.NewManager(database, cryptoSvc)
	go leaseManager.StartCleanup(ctx)
	log.Println("Lease cleanup goroutine started")

	// Create API server with all production dependencies
	serverConfig := api.ServerConfig{
		OIDCClient:        oidcClient,
		RotationScheduler: rotationScheduler,
		LeaseManager:      leaseManager,
	}
	apiServer := api.NewServerWithConfig(database, authSvc, cryptoSvc, policySvc, auditSvc, serverConfig)

	// Create HTTP server
	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      apiServer.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGTERM/SIGINT
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("TeamVault API server starting on %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutdown signal received, gracefully stopping...")

	// Stop background goroutines
	cancel()
	rotationScheduler.Stop()
	leaseManager.Stop()

	// Graceful HTTP shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
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
