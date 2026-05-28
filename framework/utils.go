package framework

import (
	"context"
	"os"
	"strconv"
	"time"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvironmentBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err == nil {
		return parsed
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(seconds) * time.Second
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
