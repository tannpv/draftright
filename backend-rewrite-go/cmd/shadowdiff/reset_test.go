package main

import (
	"strings"
	"testing"
)

func TestResetStatements_Order(t *testing.T) {
	stmts := resetStatements("draftright_shadow_go", "draftright_shadow_tmpl")
	if len(stmts) != 3 {
		t.Fatalf("want 3 statements, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "pg_terminate_backend") ||
		!strings.Contains(stmts[0], "'draftright_shadow_go'") {
		t.Fatalf("stmt[0] must terminate conns to target db: %q", stmts[0])
	}
	if stmts[1] != `DROP DATABASE IF EXISTS "draftright_shadow_go"` {
		t.Fatalf("stmt[1] = %q", stmts[1])
	}
	if stmts[2] != `CREATE DATABASE "draftright_shadow_go" TEMPLATE "draftright_shadow_tmpl"` {
		t.Fatalf("stmt[2] = %q", stmts[2])
	}
}

func TestResetStatements_QuotesIdentifiers(t *testing.T) {
	stmts := resetStatements("db-go", "tmpl")
	if !strings.Contains(stmts[1], `"db-go"`) {
		t.Fatalf("target not quoted: %q", stmts[1])
	}
}
