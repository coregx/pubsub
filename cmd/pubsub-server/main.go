// Package main provides the PubSub server executable with HTTP API and background worker.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	log.Println("🚀 Starting PubSub Server v0.1.0...")

	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("📝 Configuration loaded:")
	log.Printf("   Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("   Database: %s (%s:%d)", cfg.Database.Driver, cfg.Database.Host, cfg.Database.Port)
	log.Printf("   Worker batch size: %d", cfg.PubSub.BatchSize)
	log.Printf("   Worker interval: %ds", cfg.PubSub.WorkerInterval)

	// Connect to database
	db, err := sql.Open(cfg.Database.Driver, cfg.Database.GetDSN())
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database: %v", closeErr)
		}
	}()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("✅ Database connection established")

	// Create logger
	logger := &SimpleLogger{}

	// Create repositories using Relica adapters
	var repos *relica.Repositories
	if cfg.Database.Prefix != "" {
		repos = relica.NewRepositoriesWithPrefix(db, cfg.Database.Driver, cfg.Database.Prefix)
	} else {
		repos = relica.NewRepositories(db, cfg.Database.Driver)
	}
	log.Println("✅ Repositories initialized (Relica adapters)")

	// Create notification service
	var notificationService pubsub.NotificationService
	if cfg.PubSub.EnableNotifications {
		notificationService = pubsub.NewLoggingNotificationService(logger)
	} else {
		notificationService = &pubsub.NoOpNotificationService{}
	}

	// Create Publisher service
	publisher, err := pubsub.NewPublisher(
		pubsub.WithPublisherRepositories(repos.Message, repos.Queue, repos.Subscription, repos.Topic),
		pubsub.WithPublisherLogger(logger),
	)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	log.Println("✅ Publisher service created")

	// Create SubscriptionManager service
	subscriptionManager, err := pubsub.NewSubscriptionManager(
		pubsub.WithSubscriptionManagerRepositories(repos.Subscription, repos.Subscriber, repos.Topic),
		pubsub.WithSubscriptionManagerLogger(logger),
	)
	if err != nil {
		log.Fatalf("Failed to create subscription manager: %v", err)
	}
	log.Println("✅ SubscriptionManager service created")

	// Create QueueWorker
	worker, err := pubsub.NewQueueWorker(
		pubsub.WithRepositories(repos.Queue, repos.Message, repos.Subscription, repos.DLQ),
		pubsub.WithDelivery(nil, nil), // TODO: implement delivery provider
		pubsub.WithLogger(logger),
		pubsub.WithBatchSize(cfg.PubSub.BatchSize),
		pubsub.WithNotifications(notificationService),
	)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}
	log.Println("✅ QueueWorker created")

	// Start worker in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Printf("🔄 Starting queue worker (interval: %ds)...", cfg.PubSub.WorkerInterval)
		worker.Run(ctx, time.Duration(cfg.PubSub.WorkerInterval)*time.Second)
	}()

	// Create API handler
	handler := api.NewHandler(publisher, subscriptionManager, logger)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/publish", handler.HandlePublish)
	mux.HandleFunc("/api/v1/subscribe", handler.HandleSubscribe)
	mux.HandleFunc("/api/v1/subscriptions", handler.HandleListSubscriptions)
	mux.HandleFunc("/api/v1/subscriptions/", handler.HandleUnsubscribe) // Note trailing slash for :id
	mux.HandleFunc("/api/v1/health", handler.HandleHealth)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(mux, logger),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	go func() {
		log.Printf("🌐 HTTP server listening on %s", addr)
		log.Println("📡 API Endpoints:")
		log.Println("   POST   /api/v1/publish")
		log.Println("   POST   /api/v1/subscribe")
		log.Println("   GET    /api/v1/subscriptions")
		log.Println("   DELETE /api/v1/subscriptions/:id")
		log.Println("   GET    /api/v1/health")
		log.Println()
		log.Println("✅ PubSub Server is ready!")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	cancel() // Stop worker
	log.Println("✅ Server stopped gracefully")
}

// loggingMiddleware logs HTTP requests.
func loggingMiddleware(next http.Handler, logger pubsub.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger.Infof("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		logger.Debugf("%s %s - %v", r.Method, r.URL.Path, time.Since(start))
	})
}
