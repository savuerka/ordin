package framework

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct{ sql *sql.DB }

func ConnectPostgres(dsn string) (*DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &DB{sql: db}, nil
}

func (db *DB) Close() error { return db.sql.Close() }
func (db *DB) SQL() *sql.DB { return db.sql }
func (db *DB) Table(name string) *QueryBuilder {
	return &QueryBuilder{db: db.sql, table: name, limit: -1}
}
func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.sql.ExecContext(ctx, query, args...)
}

type QueryBuilder struct {
	db      *sql.DB
	table   string
	selects []string
	wheres  []string
	orders  []string
	args    []any
	limit   int
	offset  int
}

func (q *QueryBuilder) Select(cols ...string) *QueryBuilder { q.selects = cols; return q }
func (q *QueryBuilder) Where(cond string, args ...any) *QueryBuilder {
	q.wheres = append(q.wheres, cond)
	q.args = append(q.args, args...)
	return q
}
func (q *QueryBuilder) OrderBy(expr string) *QueryBuilder {
	q.orders = append(q.orders, expr)
	return q
}
func (q *QueryBuilder) Limit(n int) *QueryBuilder  { q.limit = n; return q }
func (q *QueryBuilder) Offset(n int) *QueryBuilder { q.offset = n; return q }

func (q *QueryBuilder) Get(ctx context.Context, dst any) error {
	query, args := q.buildSelect()
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanAll(rows, dst)
}

func (q *QueryBuilder) First(ctx context.Context, dst any) error {
	q.limit = 1
	query, args := q.buildSelect()
	row := q.db.QueryRowContext(ctx, query, args...)
	return scanRow(row, dst)
}

func (q *QueryBuilder) Insert(ctx context.Context, model any) error {
	cols, vals := columnsAndValues(model)
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	_, err := q.db.ExecContext(ctx, query, vals...)
	return err
}

func (q *QueryBuilder) Update(ctx context.Context, values map[string]any) error {
	sets := make([]string, 0, len(values))
	args := make([]any, 0, len(values)+len(q.args))
	for col, val := range values {
		sets = append(sets, fmt.Sprintf("%s = ?", col))
		args = append(args, val)
	}
	query := fmt.Sprintf("UPDATE %s SET %s", q.table, strings.Join(sets, ", "))
	if len(q.wheres) > 0 {
		query += " WHERE " + strings.Join(q.wheres, " AND ")
	}
	args = append(args, q.args...)
	query = bindPlaceholders(query)
	_, err := q.db.ExecContext(ctx, query, args...)
	return err
}

func (q *QueryBuilder) Delete(ctx context.Context) error {
	query := fmt.Sprintf("DELETE FROM %s", q.table)
	if len(q.wheres) > 0 {
		query += " WHERE " + strings.Join(q.wheres, " AND ")
	}
	_, err := q.db.ExecContext(ctx, bindPlaceholders(query), q.args...)
	return err
}

func (q *QueryBuilder) buildSelect() (string, []any) {
	cols := "*"
	if len(q.selects) > 0 {
		cols = strings.Join(q.selects, ", ")
	}
	query := fmt.Sprintf("SELECT %s FROM %s", cols, q.table)
	if len(q.wheres) > 0 {
		query += " WHERE " + strings.Join(q.wheres, " AND ")
	}
	if len(q.orders) > 0 {
		query += " ORDER BY " + strings.Join(q.orders, ", ")
	}
	if q.limit >= 0 {
		query += fmt.Sprintf(" LIMIT %d", q.limit)
	}
	if q.offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", q.offset)
	}
	return bindPlaceholders(query), q.args
}

func columnsAndValues(model any) ([]string, []any) {
	v := reflect.Indirect(reflect.ValueOf(model))
	t := v.Type()
	cols := []string{}
	vals := []any{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		col := f.Tag.Get("db")
		if col == "" || col == "-" {
			continue
		}
		cols = append(cols, col)
		vals = append(vals, v.Field(i).Interface())
	}
	return cols, vals
}

type rowScanner interface{ Scan(dest ...any) error }

func scanRow(row rowScanner, dst any) error {
	v := reflect.Indirect(reflect.ValueOf(dst))
	ptrs := fieldPointers(v)
	return row.Scan(ptrs...)
}

func scanAll(rows *sql.Rows, dst any) error {
	slice := reflect.Indirect(reflect.ValueOf(dst))
	elemType := slice.Type().Elem()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		elem := reflect.New(elemType).Elem()
		ptrs := fieldPointersByColumns(elem, cols)
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		slice.Set(reflect.Append(slice, elem))
	}
	return rows.Err()
}

func fieldPointers(v reflect.Value) []any {
	ptrs := []any{}
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).CanAddr() {
			ptrs = append(ptrs, v.Field(i).Addr().Interface())
		}
	}
	return ptrs
}

func fieldPointersByColumns(v reflect.Value, cols []string) []any {
	ptrs := make([]any, len(cols))
	for i, col := range cols {
		found := false
		for j := 0; j < v.NumField(); j++ {
			if v.Type().Field(j).Tag.Get("db") == col && v.Field(j).CanAddr() {
				ptrs[i] = v.Field(j).Addr().Interface()
				found = true
				break
			}
		}
		if !found {
			var skip any
			ptrs[i] = &skip
		}
	}
	return ptrs
}

func bindPlaceholders(query string) string {
	var b strings.Builder
	n := 1
	for _, ch := range query {
		if ch == '?' {
			b.WriteString(fmt.Sprintf("$%d", n))
			n++
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}
