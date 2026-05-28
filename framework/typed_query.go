package framework

import "context"

// TypedQuery is a small generic wrapper around QueryBuilder.
type TypedQuery[T any] struct {
	query *QueryBuilder
}

func Query[T any](db *DB, table string) *TypedQuery[T] {
	return &TypedQuery[T]{query: db.Table(table)}
}

func Repo[T any](db *DB, table string) *TypedQuery[T] {
	return Query[T](db, table)
}

func (q *TypedQuery[T]) Select(cols ...string) *TypedQuery[T] {
	q.query.Select(cols...)
	return q
}

func (q *TypedQuery[T]) Where(cond string, args ...any) *TypedQuery[T] {
	q.query.Where(cond, args...)
	return q
}

func (q *TypedQuery[T]) OrderBy(expr string) *TypedQuery[T] {
	q.query.OrderBy(expr)
	return q
}

func (q *TypedQuery[T]) Limit(n int) *TypedQuery[T] {
	q.query.Limit(n)
	return q
}

func (q *TypedQuery[T]) Offset(n int) *TypedQuery[T] {
	q.query.Offset(n)
	return q
}

func (q *TypedQuery[T]) All(ctx context.Context) ([]T, error) {
	var rows []T
	err := q.query.Get(ctx, &rows)
	return rows, err
}

func (q *TypedQuery[T]) First(ctx context.Context) (T, error) {
	var row T
	err := q.query.First(ctx, &row)
	return row, err
}

func (q *TypedQuery[T]) Create(ctx context.Context, model T) error {
	return q.query.Insert(ctx, model)
}

func (q *TypedQuery[T]) Update(ctx context.Context, values map[string]any) error {
	return q.query.Update(ctx, values)
}

func (q *TypedQuery[T]) Delete(ctx context.Context) error {
	return q.query.Delete(ctx)
}

func (q *TypedQuery[T]) Builder() *QueryBuilder {
	return q.query
}
