package repository

import (
	"embed"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunDBMigration(url string) {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		log.Fatalf("failed to create iofs driver: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, url)
	if err != nil {
		log.Fatalf("failed to start migration instance: %v", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Printf("could not get version: %v", err)
	}

	if dirty {
		log.Printf("database is dirty at version %d, forcing...", version)
		if err := m.Force(int(version)); err != nil {
			log.Fatalf("failed to force version: %v", err)
		}
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("migration completed successfully")
}
