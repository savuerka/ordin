package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Migrator struct{ db *DB }

func NewMigrator(db *DB) *Migrator { return &Migrator{db: db} }

func (m *Migrator) Run(ctx context.Context, dir string) error {
	if _, err := m.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ DEFAULT now())`); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		name := filepath.Base(file)
		var exists string
		err := m.db.SQL().QueryRowContext(ctx, "SELECT name FROM migrations WHERE name = $1", name).Scan(&exists)
		if err == nil {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		queries := strings.TrimSpace(string(content))
		if queries == "" {
			continue
		}
		tx, err := m.db.SQL().BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, queries); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO migrations(name) VALUES($1)", name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
