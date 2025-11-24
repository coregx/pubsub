// Package config provides configuration management for the PubSub standalone server.
// It loads settings from environment variables with sensible defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the PubSub server.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	PubSub   PubSubConfig
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host string
	Port int
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Driver   string // mysql, postgres, sqlite3
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Prefix   string // Table prefix (default: "pubsub_")
}

// PubSubConfig holds PubSub-specific configuration.
type PubSubConfig struct {
	BatchSize           int  // Worker batch size
	WorkerInterval      int  // Worker interval in seconds
	EnableNotifications bool // Enable notification service
}

// Load loads configuration from environment variables.
// Follows 12-factor app principles - configuration via environment.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Driver:   getEnv("DB_DRIVER", "mysql"),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 3306),
			User:     getEnv("DB_USER", "pubsub"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_NAME", "pubsub"),
			Prefix:   getEnv("DB_PREFIX", "pubsub_"),
		},
		PubSub: PubSubConfig{
			BatchSize:           getEnvInt("PUBSUB_BATCH_SIZE", 100),
			WorkerInterval:      getEnvInt("PUBSUB_WORKER_INTERVAL", 30),
			EnableNotifications: getEnvBool("PUBSUB_ENABLE_NOTIFICATIONS", true),
		},
	}

	// Validate required fields
	if cfg.Database.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD environment variable is required")
	}

	return cfg, nil
}

// GetDSN returns the database connection string based on driver.
func (c *DatabaseConfig) GetDSN() string {
	switch strings.ToLower(c.Driver) {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			c.User, c.Password, c.Host, c.Port, c.Database)
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Host, c.Port, c.User, c.Password, c.Database)
	case "sqlite3":
		return c.Database // SQLite uses file path as DSN
	default:
		return ""
	}
}

// getEnv retrieves environment variable or returns default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves environment variable as integer or returns default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool retrieves environment variable as boolean or returns default value.
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
