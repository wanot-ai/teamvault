// Package main implements a Kubernetes mutating admission webhook that injects
// TeamVault init containers into annotated pods.
//
// The webhook watches for pods with the annotation `teamvault.dev/inject: "true"`
// and injects an init container that fetches secrets from TeamVault and writes
// them to a shared volume accessible by the application containers.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	port     int
	certFile string
	keyFile  string
)

func init() {
	flag.IntVar(&port, "port", 8443, "Webhook server port")
	flag.StringVar(&certFile, "cert", "/etc/webhook/certs/tls.crt", "TLS certificate file")
	flag.StringVar(&keyFile, "key", "/etc/webhook/certs/tls.key", "TLS key file")
}

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting TeamVault Sidecar Injector webhook server")

	// Load TLS certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate: %v", err)
	}

	// Configure webhook handler
	wh := &WebhookServer{
		TeamVaultImage:   getEnv("TEAMVAULT_IMAGE", "teamvault/teamvault:latest"),
		TeamVaultAddr:    getEnv("TEAMVAULT_ADDR", "https://teamvault.default.svc:8443"),
		SecretMountPath:  getEnv("SECRET_MOUNT_PATH", "/teamvault/secrets"),
		ServiceTokenPath: getEnv("SERVICE_TOKEN_PATH", "/var/run/secrets/teamvault/token"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", wh.HandleMutate)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Listening on :%d", port)
		if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down webhook server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
