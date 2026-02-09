package rotation

import (
	"context"
	"log"
	"strings"
	"strconv"
	"time"

	"github.com/teamvault/teamvault/internal/crypto"
	"github.com/teamvault/teamvault/internal/db"
)

// Scheduler manages periodic secret rotation.
type Scheduler struct {
	database  *db.DB
	cryptoSvc *crypto.EnvelopeCrypto
	registry  *Registry
	interval  time.Duration
	stopCh    chan struct{}
}

// NewScheduler creates a new rotation scheduler.
func NewScheduler(database *db.DB, cryptoSvc *crypto.EnvelopeCrypto) *Scheduler {
	return &Scheduler{
		database:  database,
		cryptoSvc: cryptoSvc,
		registry:  NewRegistry(),
		interval:  1 * time.Minute,
		stopCh:    make(chan struct{}),
	}
}

// Registry returns the connector registry for registering custom connectors.
func (s *Scheduler) Registry() *Registry {
	return s.registry
}

// Start begins the rotation scheduler loop. Call in a goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	log.Println("Rotation scheduler started (interval: 1m)")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Rotation scheduler stopped (context cancelled)")
			return
		case <-s.stopCh:
			log.Println("Rotation scheduler stopped")
			return
		case <-ticker.C:
			s.runDueRotations(ctx)
		}
	}
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// RotateSecret performs a single rotation for a specific secret.
func (s *Scheduler) RotateSecret(ctx context.Context, secretID string) error {
	// Get the secret
	secret, err := s.database.GetSecretByID(ctx, secretID)
	if err != nil {
		return err
	}

	// Get the rotation schedule
	schedule, err := s.database.GetRotationSchedule(ctx, secretID)
	if err != nil {
		return err
	}

	return s.executeRotation(ctx, schedule, secret)
}

// runDueRotations checks for and executes any due rotations.
func (s *Scheduler) runDueRotations(ctx context.Context) {
	schedules, err := s.database.ListDueRotations(ctx)
	if err != nil {
		log.Printf("Rotation scheduler: error listing due rotations: %v", err)
		return
	}

	for _, schedule := range schedules {
		secret, err := s.database.GetSecretByID(ctx, schedule.SecretID)
		if err != nil {
			log.Printf("Rotation scheduler: error getting secret %s: %v", schedule.SecretID, err)
			if markErr := s.database.MarkRotationFailed(ctx, schedule.ID); markErr != nil {
				log.Printf("Rotation scheduler: error marking failed: %v", markErr)
			}
			continue
		}

		if err := s.executeRotation(ctx, &schedule, secret); err != nil {
			log.Printf("Rotation scheduler: error rotating secret %s: %v", secret.Path, err)
			if markErr := s.database.MarkRotationFailed(ctx, schedule.ID); markErr != nil {
				log.Printf("Rotation scheduler: error marking failed: %v", markErr)
			}
			continue
		}

		// Calculate next rotation time
		nextRotation := computeNextRotation(schedule.CronExpression)
		if err := s.database.MarkRotationCompleted(ctx, schedule.ID, nextRotation); err != nil {
			log.Printf("Rotation scheduler: error marking completed: %v", err)
		} else {
			log.Printf("Rotation scheduler: rotated secret %s, next at %s", secret.Path, nextRotation.Format(time.RFC3339))
		}
	}
}

// executeRotation runs the connector and stores the new secret version.
func (s *Scheduler) executeRotation(ctx context.Context, schedule *db.RotationSchedule, secret *db.Secret) error {
	connector, err := s.registry.Get(schedule.ConnectorType)
	if err != nil {
		return err
	}

	// Generate new value
	newValue, err := connector.Rotate(ctx, schedule.ConnectorConfig)
	if err != nil {
		return err
	}

	// Encrypt the new value
	encrypted, err := s.cryptoSvc.Encrypt([]byte(newValue))
	if err != nil {
		return err
	}

	// Get next version
	nextVersion, err := s.database.GetNextSecretVersion(ctx, secret.ID)
	if err != nil {
		return err
	}

	// Store new version (system actor)
	_, err = s.database.CreateSecretVersion(ctx, secret.ID, nextVersion,
		encrypted.Ciphertext, encrypted.Nonce,
		encrypted.EncryptedDEK, encrypted.DEKNonce,
		encrypted.MasterKeyVersion, "system:rotation")
	return err
}

// ComputeNextRotationExported is the exported version for use by API handlers.
func ComputeNextRotationExported(cronExpr string) time.Time {
	return computeNextRotation(cronExpr)
}

// computeNextRotation parses a simple cron-like expression and computes the next run time.
// Supports: "@every <duration>" format (e.g., "@every 24h", "@every 1h30m")
// and simple interval format: "0 * * * *" (hourly), "0 0 * * *" (daily).
func computeNextRotation(cronExpr string) time.Time {
	now := time.Now().UTC()

	// Handle @every format
	if strings.HasPrefix(cronExpr, "@every ") {
		durationStr := strings.TrimPrefix(cronExpr, "@every ")
		d, err := time.ParseDuration(durationStr)
		if err == nil && d > 0 {
			return now.Add(d)
		}
	}

	// Handle simple cron-like intervals
	parts := strings.Fields(cronExpr)
	if len(parts) == 5 {
		// minute hour dom month dow
		minute, _ := strconv.Atoi(parts[0])
		hour := parts[1]

		if hour == "*" {
			// Hourly: run at :minute of next hour
			next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), minute, 0, 0, time.UTC)
			if !next.After(now) {
				next = next.Add(1 * time.Hour)
			}
			return next
		}

		hourInt, err := strconv.Atoi(hour)
		if err == nil {
			// Daily: run at hour:minute
			next := time.Date(now.Year(), now.Month(), now.Day(), hourInt, minute, 0, 0, time.UTC)
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			return next
		}
	}

	// Default: 24 hours from now
	return now.Add(24 * time.Hour)
}
