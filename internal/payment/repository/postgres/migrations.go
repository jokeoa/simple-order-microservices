package postgres

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrations() fs.FS {
	filesystem, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		log.Fatalf("load payment migrations: %v", err)
	}

	return filesystem
}
