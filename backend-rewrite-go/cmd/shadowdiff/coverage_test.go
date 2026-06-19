package main

import "testing"

func TestMissingRoutes_ReportsGaps(t *testing.T) {
	routes := []string{"POST /auth/login", "GET /plans", "GET /health"}
	fixtures := []fixture{
		{Method: "POST", Path: "/auth/login"},
		{Method: "GET", Path: "/health"},
	}
	missing := missingRoutes(routes, fixtures)
	if len(missing) != 1 || missing[0] != "GET /plans" {
		t.Fatalf("missing = %v, want [GET /plans]", missing)
	}
}

func TestMissingRoutes_PathParamsMatchByPattern(t *testing.T) {
	routes := []string{"GET /admin/users/{id}"}
	fixtures := []fixture{{Method: "GET", Path: "/admin/users/abc-123"}}
	if m := missingRoutes(routes, fixtures); len(m) != 0 {
		t.Fatalf("param route should be covered, got missing %v", m)
	}
}

func TestMissingRoutes_None(t *testing.T) {
	routes := []string{"GET /health"}
	fixtures := []fixture{{Method: "GET", Path: "/health"}}
	if m := missingRoutes(routes, fixtures); len(m) != 0 {
		t.Fatalf("want none, got %v", m)
	}
}
