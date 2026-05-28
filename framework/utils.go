package framework

import (
	"context"
	"os"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func MustPostgres(dsn string) *DB {
	db, err := ConnectPostgres(dsn)
	if err != nil {
		panic(err)
	}
	return db
}

func MustPostgresEnv(key, fallback string) *DB {
	return MustPostgres(getenv(key, fallback))
}

func MustMigrate(db *DB, dir string) {
	if err := NewMigrator(db).Run(context.Background(), dir); err != nil {
		panic(err)
	}
}
