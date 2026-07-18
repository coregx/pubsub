// Package main provides the PubSub server executable with HTTP API and background worker.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/coregx/fursy"
	"github.com/coregx/fursy/middleware"
	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/adapters/relica"
	"github.com/coregx/pubsub/cmd/pubsub-server/internal/api"
	"github.com/coregx/pubsub/cmd/pubsub-server/internal/config"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// SimpleLogger implements pubsub.Logger for standard logging.
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
func (l *SimpleLogger) Info(message string) {
	log.Printf("[INFO] %s", message)
}

func main() {
	log.Println("Starting PubSub Server v0.2.0...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded:")
	log.Printf("   Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("   Database: %s (%s:%d)", cfg.Database.Driver, cfg.Database.Host, cfg.Database.Port)
	log.Printf("   Worker batch size: %d", cfg.PubSub.BatchSize)
	log.Printf("   Worker interval: %ds", cfg.PubSub.WorkerInterval)

	db, err := sql.Open(cfg.Database.Driver, cfg.Database.GetDSN())
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established")

	logger := &SimpleLogger{}

	var repos *relica.Repositories
	if cfg.Database.Prefix != "" {
		repos = relica.NewRepositoriesWithPrefix(db, cfg.Database.Driver, cfg.Database.Prefix)
	} else {
		repos = relica.NewRepositories(db, cfg.Database.Driver)
	}
	log.Println("Repositories initialized (Relica adapters)")

	var notificationService pubsub.NotificationService
	if cfg.PubSub.EnableNotifications {
		notificationService = pubsub.NewLoggingNotificationService(logger)
	} else {
		notificationService = &pubsub.NoOpNotificationService{}
	}

	publisher, err := pubsub.NewPublisher(
		pubsub.WithPublisherRepositories(repos.Message, repos.Queue, repos.Subscription, repos.Topic),
		pubsub.WithPublisherLogger(logger),
	)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}

	subscriptionManager, err := pubsub.NewSubscriptionManager(
		pubsub.WithSubscriptionManagerRepositories(repos.Subscription, repos.Subscriber, repos.Topic),
		pubsub.WithSubscriptionManagerLogger(logger),
	)
	if err != nil {
		log.Fatalf("Failed to create subscription manager: %v", err)
	}

	worker, err := pubsub.NewQueueWorker(
		pubsub.WithRepositories(repos.Queue, repos.Message, repos.Subscription, repos.DLQ),
		pubsub.WithDelivery(nil, nil),
		pubsub.WithLogger(logger),
		pubsub.WithBatchSize(cfg.PubSub.BatchSize),
		pubsub.WithNotifications(notificationService),
	)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Printf("Starting queue worker (interval: %ds)...", cfg.PubSub.WorkerInterval)
		worker.Run(ctx, time.Duration(cfg.PubSub.WorkerInterval)*time.Second)
	}()

	router := fursy.New().
		WithTrailingSlash(fursy.StripTrailingSlash).
		WithInfo(fursy.Info{
			Title:       "PubSub API",
			Version:     "0.2.0",
			Description: "Production-ready Pub/Sub service with reliable message delivery",
		})

	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())

	handler := api.NewHandler(publisher, subscriptionManager, logger)
	handler.RegisterRoutes(router)

	router.OnShutdown(func() {
		cancel()
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database: %v", closeErr)
		}
		log.Println("Server stopped gracefully")
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("HTTP server listening on %s", addr)
	log.Println("API Endpoints:")
	log.Println("   POST   /api/v1/publish")
	log.Println("   POST   /api/v1/subscribe")
	log.Println("   GET    /api/v1/subscriptions")
	log.Println("   DELETE /api/v1/subscriptions/:id")
	log.Println("   GET    /api/v1/health")
	log.Println("PubSub Server is ready!")

	if err := router.ListenAndServeWithShutdown(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
