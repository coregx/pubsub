// Package config provides configuration management for the PubSub standalone server.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allEnvKeys lists every environment variable read by Load so tests can clear
// them all before asserting defaults.
var allEnvKeys = []string{
	"SERVER_HOST",
	"SERVER_PORT",
	"DB_DRIVER",
	"DB_HOST",
	"DB_PORT",
	"DB_USER",
	"DB_PASSWORD",
	"DB_NAME",
	"DB_PREFIX",
	"PUBSUB_BATCH_SIZE",
	"PUBSUB_WORKER_INTERVAL",
	"PUBSUB_ENABLE_NOTIFICATIONS",
}

// clearEnv resets all config-related environment variables to empty strings so
// each test starts from a clean state.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range allEnvKeys {
		t.Setenv(key, "")
	}
}

// TestLoad_DefaultValues verifies that Load returns the expected default values
// when only the required DB_PASSWORD variable is provided.
func TestLoad_DefaultValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_PASSWORD", "test")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Server defaults.
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)

	// Database defaults.
	assert.Equal(t, "mysql", cfg.Database.Driver)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 3306, cfg.Database.Port)
	assert.Equal(t, "pubsub", cfg.Database.User)
	assert.Equal(t, "test", cfg.Database.Password)
	assert.Equal(t, "pubsub", cfg.Database.Database)
	assert.Equal(t, "pubsub_", cfg.Database.Prefix)

	// PubSub defaults.
	assert.Equal(t, 100, cfg.PubSub.BatchSize)
	assert.Equal(t, 30, cfg.PubSub.WorkerInterval)
	assert.True(t, cfg.PubSub.EnableNotifications)
}

// TestLoad_MissingPassword verifies that Load returns an error when DB_PASSWORD
// is not provided.
func TestLoad_MissingPassword(t *testing.T) {
	clearEnv(t)
	// DB_PASSWORD intentionally not set.

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DB_PASSWORD")
}

// TestLoad_CustomValues verifies that Load correctly applies all custom
// environment variable overrides.
func TestLoad_CustomValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("DB_PREFIX", "app_")
	t.Setenv("PUBSUB_BATCH_SIZE", "200")
	t.Setenv("PUBSUB_WORKER_INTERVAL", "60")
	t.Setenv("PUBSUB_ENABLE_NOTIFICATIONS", "false")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "admin", cfg.Database.User)
	assert.Equal(t, "secret", cfg.Database.Password)
	assert.Equal(t, "mydb", cfg.Database.Database)
	assert.Equal(t, "app_", cfg.Database.Prefix)
	assert.Equal(t, 200, cfg.PubSub.BatchSize)
	assert.Equal(t, 60, cfg.PubSub.WorkerInterval)
	assert.False(t, cfg.PubSub.EnableNotifications)
}

// TestLoad_InvalidIntFallsToDefault verifies that an unparseable integer
// environment variable causes getEnvInt to fall back to the default value.
func TestLoad_InvalidIntFallsToDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("SERVER_PORT", "not-a-number")
	t.Setenv("DB_PORT", "abc")
	t.Setenv("PUBSUB_BATCH_SIZE", "xyz")
	t.Setenv("PUBSUB_WORKER_INTERVAL", "!!")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 3306, cfg.Database.Port)
	assert.Equal(t, 100, cfg.PubSub.BatchSize)
	assert.Equal(t, 30, cfg.PubSub.WorkerInterval)
}

// TestLoad_InvalidBoolFallsToDefault verifies that an unparseable boolean
// environment variable causes getEnvBool to fall back to the default value.
func TestLoad_InvalidBoolFallsToDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("PUBSUB_ENABLE_NOTIFICATIONS", "not-a-bool")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Default is true; invalid value must not change it.
	assert.True(t, cfg.PubSub.EnableNotifications)
}

// TestLoad_NotificationsExplicitTrue verifies that "true" explicitly enables
// notifications.
func TestLoad_NotificationsExplicitTrue(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("PUBSUB_ENABLE_NOTIFICATIONS", "true")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.True(t, cfg.PubSub.EnableNotifications)
}

// TestLoad_NotificationsExplicitFalse verifies that "false" disables
// notifications.
func TestLoad_NotificationsExplicitFalse(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("PUBSUB_ENABLE_NOTIFICATIONS", "false")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.False(t, cfg.PubSub.EnableNotifications)
}

// TestGetDSN_MySQL verifies the MySQL connection string format produced by
// GetDSN.
func TestGetDSN_MySQL(t *testing.T) {
	dbCfg := DatabaseConfig{
		Driver:   "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "user",
		Password: "pass",
		Database: "testdb",
	}

	dsn := dbCfg.GetDSN()

	assert.Contains(t, dsn, "user:pass")
	assert.Contains(t, dsn, "tcp(localhost:3306)")
	assert.Contains(t, dsn, "testdb")
	assert.Contains(t, dsn, "parseTime=true")
	// Verify exact format: user:pass@tcp(host:port)/db?parseTime=true
	assert.Equal(t, "user:pass@tcp(localhost:3306)/testdb?parseTime=true", dsn)
}

// TestGetDSN_MySQL_CaseInsensitive verifies that driver name matching is
// case-insensitive.
func TestGetDSN_MySQL_CaseInsensitive(t *testing.T) {
	dbCfg := DatabaseConfig{
		Driver:   "MySQL",
		Host:     "localhost",
		Port:     3306,
		User:     "user",
		Password: "pass",
		Database: "testdb",
	}

	dsn := dbCfg.GetDSN()

	assert.Contains(t, dsn, "parseTime=true")
}

// TestGetDSN_PostgreSQL verifies the PostgreSQL connection string format
// produced by GetDSN.
func TestGetDSN_PostgreSQL(t *testing.T) {
	dbCfg := DatabaseConfig{
		Driver:   "postgres",
		Host:     "pg.example.com",
		Port:     5432,
		User:     "pguser",
		Password: "pgpass",
		Database: "pgdb",
	}

	dsn := dbCfg.GetDSN()

	assert.Contains(t, dsn, "host=pg.example.com")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=pguser")
	assert.Contains(t, dsn, "password=pgpass")
	assert.Contains(t, dsn, "dbname=pgdb")
	assert.Contains(t, dsn, "sslmode=disable")
	assert.Equal(t, "host=pg.example.com port=5432 user=pguser password=pgpass dbname=pgdb sslmode=disable", dsn)
}

// TestGetDSN_SQLite3 verifies that GetDSN returns the file path as-is for
// SQLite3, since SQLite uses the database name as the DSN.
func TestGetDSN_SQLite3(t *testing.T) {
	dbCfg := DatabaseConfig{
		Driver:   "sqlite3",
		Database: "/var/lib/pubsub/pubsub.db",
	}

	dsn := dbCfg.GetDSN()

	assert.Equal(t, "/var/lib/pubsub/pubsub.db", dsn)
}

// TestGetDSN_UnknownDriver verifies that an unrecognized driver returns an
// empty string.
func TestGetDSN_UnknownDriver(t *testing.T) {
	dbCfg := DatabaseConfig{
		Driver:   "mssql",
		Host:     "localhost",
		Port:     1433,
		User:     "sa",
		Password: "pass",
		Database: "db",
	}

	dsn := dbCfg.GetDSN()

	assert.Empty(t, dsn)
}
