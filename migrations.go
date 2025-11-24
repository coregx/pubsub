package pubsub

import "embed"

// MigrationFiles contains all SQL migration files embedded in the binary.
// Users can access these files programmatically to apply migrations using
// their preferred migration tool (goose, golang-migrate, atlas, etc.)
//
// Example with goose:
//
//	import (
//	    "github.com/pressly/goose/v3"
//	    pubsub "github.com/coregx/pubsub"
//	)
//
//	goose.SetBaseFS(pubsub.MigrationFiles)
//	if err := goose.Up(db, "migrations"); err != nil {
//	    log.Fatal(err)
//	}
//
// Example with golang-migrate:
//
//	import (
//	    "github.com/golang-migrate/migrate/v4"
//	    _ "github.com/golang-migrate/migrate/v4/database/mysql"
//	    "github.com/golang-migrate/migrate/v4/source/iofs"
//	    pubsub "github.com/coregx/pubsub"
//	)
//
//	source, err := iofs.New(pubsub.MigrationFiles, "migrations")
//	m, err := migrate.NewWithSourceInstance("iofs", source, "mysql://user:pass@tcp(host:port)/db")
//	m.Up()
//
//go:embed migrations/*.sql
var MigrationFiles embed.FS
