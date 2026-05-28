package framework

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	sql *sql.DB
	dsn string
}

func ConnectPostgres(dsn string) (*DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &DB{sql: db, dsn: dsn}, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) SQL() *sql.DB {
	return db.sql
}

func (db *DB) DSN() string {
	return db.dsn
}

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

func (q *QueryBuilder) Select(cols ...string) *QueryBuilder {
	q.selects = cols
	return q
}

func (q *QueryBuilder) Where(cond string, args ...any) *QueryBuilder {
	q.wheres = append(q.wheres, cond)
	q.args = append(q.args, args...)
	return q
}

func (q *QueryBuilder) OrderBy(expr string) *QueryBuilder {
	q.orders = append(q.orders, expr)
	return q
}

func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = n
	return q
}

func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offset = n
	return q
}

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
		col, options := parseDBTag(f.Tag.Get("db"))
		if col == "" || col == "-" {
			continue
		}
		fieldValue := v.Field(i)
		if options["omitempty"] && fieldValue.IsZero() {
			continue
		}
		cols = append(cols, col)
		vals = append(vals, fieldValue.Interface())
	}

	return cols, vals
}

func parseDBTag(tag string) (string, map[string]bool) {
	parts := strings.Split(tag, ",")
	name := strings.TrimSpace(parts[0])
	options := make(map[string]bool, len(parts)-1)
	for _, option := range parts[1:] {
		option = strings.TrimSpace(option)
		if option != "" {
			options[option] = true
		}
	}
	return name, options
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(row rowScanner, dst any) error {
	v := reflect.Indirect(reflect.ValueOf(dst))
	fields := dbFieldsInStructOrder(v)
	values, ptrs := rawScanTargets(len(fields))
	if err := row.Scan(ptrs...); err != nil {
		return err
	}
	return assignScannedValues(v, fields, values)
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
		fields := dbFieldsByColumns(elem, cols)
		values, ptrs := rawScanTargets(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if err := assignScannedValues(elem, fields, values); err != nil {
			return err
		}
		slice.Set(reflect.Append(slice, elem))
	}
	return rows.Err()
}

func rawScanTargets(n int) ([]any, []any) {
	values := make([]any, n)
	ptrs := make([]any, n)
	for i := range values {
		ptrs[i] = &values[i]
	}
	return values, ptrs
}

func dbFieldsInStructOrder(v reflect.Value) []int {
	fields := make([]int, 0, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).CanSet() {
			continue
		}
		if tag, _ := parseDBTag(v.Type().Field(i).Tag.Get("db")); tag == "-" {
			continue
		}
		fields = append(fields, i)
	}
	return fields
}

func dbFieldsByColumns(v reflect.Value, cols []string) []int {
	fields := make([]int, len(cols))
	for i := range fields {
		fields[i] = -1
	}

	for i, col := range cols {
		for j := 0; j < v.NumField(); j++ {
			field := v.Field(j)
			structField := v.Type().Field(j)
			if !field.CanSet() {
				continue
			}
			tag, _ := parseDBTag(structField.Tag.Get("db"))
			if tag == "-" {
				continue
			}
			if tag == col || (tag == "" && strings.EqualFold(structField.Name, col)) {
				fields[i] = j
				break
			}
		}
	}
	return fields
}

func assignScannedValues(v reflect.Value, fields []int, values []any) error {
	for i, fieldIndex := range fields {
		if fieldIndex < 0 || fieldIndex >= v.NumField() {
			continue
		}
		if i >= len(values) {
			break
		}
		if err := assignScannedValue(v.Field(fieldIndex), values[i]); err != nil {
			name := v.Type().Field(fieldIndex).Name
			return fmt.Errorf("scan field %s: %w", name, err)
		}
	}
	return nil
}

func assignScannedValue(field reflect.Value, value any) error {
	if !field.CanSet() {
		return nil
	}
	if field.CanAddr() {
		if scanner, ok := field.Addr().Interface().(sql.Scanner); ok {
			return scanner.Scan(value)
		}
	}
	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}
	if field.Kind() == reflect.Pointer {
		elem := reflect.New(field.Type().Elem())
		if scanner, ok := elem.Interface().(sql.Scanner); ok {
			if err := scanner.Scan(value); err != nil {
				return err
			}
			field.Set(elem)
			return nil
		}
		if err := assignScannedValue(elem.Elem(), value); err != nil {
			return err
		}
		field.Set(elem)
		return nil
	}
	if valueBytes, ok := value.([]byte); ok {
		value = string(valueBytes)
	}
	if valueTime, ok := value.(time.Time); ok && field.Type() == reflect.TypeOf(time.Time{}) {
		field.Set(reflect.ValueOf(valueTime))
		return nil
	}

	val := reflect.ValueOf(value)
	if val.IsValid() && val.Type().AssignableTo(field.Type()) {
		field.Set(val)
		return nil
	}
	if val.IsValid() && val.Type().ConvertibleTo(field.Type()) {
		field.Set(val.Convert(field.Type()))
		return nil
	}

	s := fmt.Sprint(value)
	switch field.Kind() {
	case reflect.String:
		field.SetString(s)
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		field.SetBool(b)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(s)
			if err != nil {
				return err
			}
			field.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(s, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(n)
		return nil
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(n)
		return nil
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.Uint8 {
			field.SetBytes([]byte(s))
			return nil
		}
	}
	return fmt.Errorf("cannot assign database value %T to %s", value, field.Type())
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
