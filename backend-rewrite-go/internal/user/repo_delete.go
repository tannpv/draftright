package user

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// execer runs one statement. Both pgx.Tx (via the adapter) and the
// recording test fake satisfy it, so the cascade is unit-tested without
// a database.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) error
}

// DeleteExecer is the production seam: something that can open a
// transaction. *pgxpool.Pool satisfies it.
type DeleteExecer interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// deleteAccountStmts runs the 8-statement cascade in the exact order the
// NestJS usersService.deleteAccount uses. Any error aborts immediately
// (caller rolls back).
func deleteAccountStmts(ctx context.Context, e execer, userID string) error {
	stmts := []string{
		`DELETE FROM extension_tokens WHERE user_id = $1`,
		`DELETE FROM payments WHERE user_id = $1`,
		`DELETE FROM usage_logs WHERE user_id = $1`,
		`DELETE FROM subscriptions WHERE user_id = $1`,
		`DELETE FROM feature_votes WHERE user_id = $1`,
		`UPDATE bug_reports SET user_id = NULL WHERE user_id = $1`,
		`UPDATE error_reports SET user_id = NULL WHERE user_id = $1`,
		`DELETE FROM users WHERE id = $1`,
	}
	for _, s := range stmts {
		if err := e.Exec(ctx, s, userID); err != nil {
			return err
		}
	}
	return nil
}

// txExecer adapts a pgx.Tx to the execer interface (pgx.Tx.Exec returns
// a CommandTag we discard).
type txExecer struct{ tx pgx.Tx }

func (t txExecer) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := t.tx.Exec(ctx, sql, args...)
	return err
}

// DeleteAccount runs the cascade inside one transaction.
func (r *PgRepo) DeleteAccount(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := deleteAccountStmts(ctx, txExecer{tx}, id); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Compile-time assertion that *pgxpool.Pool is a valid DeleteExecer.
var _ DeleteExecer = (*pgxpool.Pool)(nil)
