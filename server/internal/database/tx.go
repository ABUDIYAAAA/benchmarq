package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the query surface shared by *pgxpool.Pool and pgx.Tx, so repository
// code is identical whether or not it runs inside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// Transactor runs a unit of work atomically. Services depend on this interface
// rather than on pgx, which keeps them unit-testable.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txKey struct{}

type PgTransactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *PgTransactor {
	return &PgTransactor{pool: pool}
}

// WithinTx begins a transaction and stores it in the context passed to fn.
// Repositories pick it up through Conn. Nested calls join the outer
// transaction. The transaction is rolled back if fn returns an error or panics.
func (t *PgTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// Conn returns the transaction bound to ctx, or the pool when there is none.
func Conn(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// InTx reports whether ctx carries an active transaction.
func InTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(pgx.Tx)
	return ok
}

// NoopTransactor runs fn directly. Intended for unit tests with fake repositories.
type NoopTransactor struct{}

func (NoopTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// ErrNoRows re-exports pgx.ErrNoRows so callers need not import pgx just for it.
var ErrNoRows = pgx.ErrNoRows

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
