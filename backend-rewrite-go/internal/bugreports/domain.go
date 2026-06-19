// Package bugreports ingests user-submitted bug reports (POST /bug-reports,
// multipart/form-data with an optional screenshot) and serves the admin
// triage routes (list/get/screenshot/patch/delete/fix-proposal) plus the
// hourly AI fix-proposal cron. Byte-identical port of the NestJS bug-reports
// module. The feedback board + voting live in the feedback module.
package bugreports

import (
	"encoding/json"
	"errors"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// ErrNotFound is returned by the admin repo/use case when a bug_reports row is
// absent. Node BugReportsService.findById throws NotFoundException('bug report
// not found') → HTTP 404 (unlike errreport, which maps not-found to 400). The
// admin handler translates this to a 404 envelope.
var ErrNotFound = errors.New("bug report not found")

// bugAllowedStatuses mirrors Node ALLOWED_STATUSES (status filter + the PATCH
// status validation). Order is the Node literal order so the joined error
// message ("status must be one of: ...") matches byte-for-byte.
var bugAllowedStatuses = []string{"new", "reviewing", "fix_proposed", "resolved", "wont_fix"}

// bugTargetPlatforms mirrors Node TARGET_PLATFORMS (the list + PATCH
// target_platform validation). Order is the Node literal order so the joined
// error message ("target_platform must be one of: ...") matches byte-for-byte.
var bugTargetPlatforms = []string{"playground", "mobile", "windows", "mac", "linux"}

// Created is the InsertBugReport projection the handler stamps on the row.
// display_no is a bigint sequence in Postgres → int64.
type Created struct {
	ID        string
	DisplayNo int64
}

// BugReportEntity is the full bug_reports row returned by the admin
// read/get/patch/delete/fix-proposal routes. JSON key order mirrors the Node
// entity (src/bug-reports/entities/bug-report.entity.ts) exactly so the
// marshalled bytes match. The order is fixed by the explicit struct tags:
//
//	id, display_no, source, description, screenshot_path, screenshot_filename,
//	app_version, os_info, user_id, user_email, context, status, kind, title,
//	target_platform, vote_count, is_public, admin_notes, ai_fix_proposal,
//	ai_fix_proposed_at, created_at, updated_at
//
// Parity notes (mirroring the proven errreport entity handling):
//   - display_no is a TypeORM bigint → the `pg` driver returns it as a JS
//     STRING (no setTypeParser override in the Node codebase), so
//     JSON.stringify emits "display_no":"203" (quoted). Held as a Go string,
//     NOT int — the plan's *int sketch would emit an unquoted number and
//     break shadow parity.
//   - created_at / updated_at / ai_fix_proposed_at are TypeORM Date columns →
//     serialized via Date.toISOString() (UTC, exactly 3 fractional digits,
//     trailing Z). Held as ISOMillis-formatted strings; nullable ones (the
//     latter) are *string so they emit JSON null when absent.
//   - context is jsonb marshalled raw so null/{} round-trip byte-identically.
type BugReportEntity struct {
	ID                 string          `json:"id"`
	DisplayNo          string          `json:"display_no"`
	Source             string          `json:"source"`
	Description        string          `json:"description"`
	ScreenshotPath     *string         `json:"screenshot_path"`
	ScreenshotFilename *string         `json:"screenshot_filename"`
	AppVersion         *string         `json:"app_version"`
	OsInfo             *string         `json:"os_info"`
	UserID             *string         `json:"user_id"`
	UserEmail          *string         `json:"user_email"`
	Context            json.RawMessage `json:"context"`
	Status             string          `json:"status"`
	Kind               string          `json:"kind"`
	Title              *string         `json:"title"`
	TargetPlatform     *string         `json:"target_platform"`
	VoteCount          int             `json:"vote_count"`
	IsPublic           bool            `json:"is_public"`
	AdminNotes         *string         `json:"admin_notes"`
	AiFixProposal      *string         `json:"ai_fix_proposal"`
	AiFixProposedAt    *string         `json:"ai_fix_proposed_at"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
}

// AdminListFilter carries the parseListQuery output (Built) plus the three
// custom filters for the admin bug-report list (Node findAllPaginated). The
// repo composes the dynamic WHERE from Built + these.
type AdminListFilter struct {
	Built          listquery.Built
	Status         string // "" = no filter; 'all' = no filter; else one of ALLOWED_STATUSES
	Kind           string // "" = no filter; else 'bug' | 'feature'
	TargetPlatform string // "" = no filter; else one of bugTargetPlatforms
}
