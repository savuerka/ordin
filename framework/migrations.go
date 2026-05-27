package framework

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
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

		if err := m.runFile(ctx, file); err != nil {
			return fmt.Errorf("migration %s failed: %w", name, err)
		}

		if _, err := m.db.Exec(ctx, "INSERT INTO migrations(name) VALUES($1)", name); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) runFile(ctx context.Context, file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(content)) == "" {
		return nil
	}

	// Fast path for ordinary migrations.
	// PostgreSQL dumps with COPY ... FROM stdin need COPY protocol, not plain Exec.
	if !bytes.Contains(bytes.ToUpper(content), []byte("FROM STDIN")) {
		tx, err := m.db.SQL().BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}

	return m.runPostgresDump(ctx, file)
}

func (m *Migrator) runPostgresDump(ctx context.Context, file string) error {
	conn, err := pgx.Connect(ctx, m.db.DSN())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 1024*1024)
	var stmt strings.Builder

	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)

			if strings.HasPrefix(trimmed, "--") || trimmed == "" {
				// Keep comments harmless and avoid executing comment-only statements.
				continue
			}

			stmt.WriteString(line)

			if isCopyFromStdin(stmt.String()) {
				copySQL := strings.TrimSpace(stmt.String())
				stmt.Reset()

				data, readErr := readCopyData(r)
				if readErr != nil {
					return readErr
				}

				if _, copyErr := conn.PgConn().CopyFrom(ctx, bytes.NewReader(data), copySQL); copyErr != nil {
					return copyErr
				}
				continue
			}

			if statementLooksComplete(stmt.String()) {
				sql := strings.TrimSpace(stmt.String())
				stmt.Reset()
				if sql != "" {
					if _, execErr := conn.Exec(ctx, sql); execErr != nil {
						return execErr
					}
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if sql := strings.TrimSpace(stmt.String()); sql != "" {
		_, err := conn.Exec(ctx, sql)
		return err
	}

	return nil
}

func isCopyFromStdin(sql string) bool {
	s := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(s, "COPY ") && strings.Contains(s, " FROM STDIN") && strings.HasSuffix(s, ";")
}

func readCopyData(r *bufio.Reader) ([]byte, error) {
	var b bytes.Buffer
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			if strings.TrimRight(line, "\r\n") == `\.` {
				return b.Bytes(), nil
			}
			b.WriteString(line)
		}
		if err == io.EOF {
			return nil, fmt.Errorf("unexpected EOF while reading COPY data")
		}
		if err != nil {
			return nil, err
		}
	}
}

func statementLooksComplete(sql string) bool {
	s := strings.TrimSpace(sql)
	if s == "" || !strings.HasSuffix(s, ";") {
		return false
	}
	return !insideSingleQuote(s) && !insideDollarQuote(s)
}

func insideSingleQuote(s string) bool {
	in := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			if i+1 < len(s) && s[i+1] == '\'' {
				i++
				continue
			}
			in = !in
		}
	}
	return in
}

func insideDollarQuote(s string) bool {
	var tag string
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			continue
		}
		j := i + 1
		for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
			j++
		}
		if j < len(s) && s[j] == '$' {
			candidate := s[i : j+1]
			if tag == "" {
				tag = candidate
			} else if tag == candidate {
				tag = ""
			}
			i = j
		}
	}
	return tag != ""
}
