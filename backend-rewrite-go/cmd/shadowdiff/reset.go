package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// resetStatements returns the ordered SQL that drops `target` and recreates it
// from `template`. Statement order is load-bearing: terminate live sessions
// FIRST (cheap, kills most pool conns), then drop WITH (FORCE), then clone.
// The plain terminate alone races a pgxpool that re-dials instantly — a new
// connection can land between the terminate and the DROP, yielding
// "database ... is being accessed by other users" (SQLSTATE 55006). DROP
// DATABASE ... WITH (FORCE) (PG13+) terminates remaining sessions and drops in
// one atomic statement, closing that window. Identifiers are double-quoted; the
// datname literal in the terminate filter is single-quoted (it is a string, not
// an identifier).
func resetStatements(target, template string) []string {
	return []string{
		fmt.Sprintf(
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()",
			target),
		fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, target),
		fmt.Sprintf(`CREATE DATABASE %q TEMPLATE %q`, target, template),
	}
}

// resetDB runs resetStatements against a maintenance connection (which MUST be
// connected to a different database than `target`, e.g. "postgres"). Used live
// in the gate runner; not unit-tested (needs a real server).
func resetDB(ctx context.Context, maint *pgx.Conn, target, template string) error {
	for _, s := range resetStatements(target, template) {
		if _, err := maint.Exec(ctx, s); err != nil {
			return fmt.Errorf("reset %s: %q: %w", target, s, err)
		}
	}
	return nil
}
