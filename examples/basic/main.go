package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	pubsub "github.com/coregx/pubsub"
	"github.com/coregx/pubsub/adapters/relica"
	_ "github.com/go-sql-driver/mysql"
)

// Example Logger adapter
type SimpleLogger struct{}

func (l *SimpleLogger) Debugf(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}
func (l *SimpleLogger) Infof(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}
func (l *SimpleLogger) Warnf(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}
func (l *SimpleLogger) Errorf(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}
func (l *SimpleLogger) Info(message string) { log.Printf("[INFO] %s", message) }

func main() {
	// Connect to database
	db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/pubsub_db?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Create logger
	logger := &SimpleLogger{}

	// Create repositories using Relica adapters (production-ready!)
	// driverName should be "mysql", "postgres", or "sqlite3"
	repos := relica.NewRepositories(db, "mysql")

	// Create worker using Options Pattern (2025 best practice)
	worker, err := pubsub.NewQueueWorker(
		pubsub.WithRepositories(
			repos.Queue,
			repos.Message,
			repos.Subscription,
			repos.DLQ,
		),
		pubsub.WithDelivery(
			nil, // transmitterProvider (TODO: implement delivery provider)
			nil, // deliveryGateway (TODO: implement HTTP/webhook gateway)
		),
		pubsub.WithLogger(logger),
		pubsub.WithBatchSize(100), // optional: customize batch size (default: 100)
		// Optional: customize notification service
		pubsub.WithNotifications(pubsub.NewLoggingNotificationService(logger)),
	)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	// Run worker
	ctx := context.Background()
	fmt.Println("Starting PubSub worker with Relica adapters...")
	fmt.Println("✅ Using production-ready Relica repository implementations")
	fmt.Println("✅ Zero production dependencies (except database driver)")
	fmt.Println("✅ Type-safe query building")
	fmt.Println("✅ Supports MySQL, PostgreSQL, and SQLite")
	fmt.Println()

	// Run worker loop (processes pending/retryable items every 30 seconds)
	worker.Run(ctx, 30*time.Second)
}
