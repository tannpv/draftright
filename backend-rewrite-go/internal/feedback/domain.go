// Package feedback powers the public upvote board (POST /feedback,
// GET /feedback, POST /feedback/:id/vote). Byte-identical port of the NestJS
// createFeedback / toggleVote / listPublicFeatures paths in
// bug-reports.service.ts. It shares the bug_reports table with the bugreports
// module (kind='feature' vs 'bug') plus a feature_votes table (one vote per
// user per feature; bug_reports.vote_count is derived = COUNT(feature_votes)).
//
// Handlers, the honeypot, and the admin board are out of scope here (Task 10+).
package feedback

import "errors"

// TargetPlatforms mirrors TARGET_PLATFORMS in create-feedback.dto.ts — the
// five platforms a feature request can target. Order is the Node array order
// (it surfaces verbatim in the ErrBadTargetPlatform message).
var TargetPlatforms = []string{"playground", "mobile", "windows", "mac", "linux"}

// Service-level sentinel errors. These are the SECOND guard, distinct from the
// DTO @IsIn/@MaxLength checks (Task 10). Messages are byte-identical to the
// BadRequestException / NotFoundException strings in createFeedback /
// findFeatureById. Mapped to 400 / 404 by the handler.
var (
	// ErrDescriptionRequired / ErrSourceRequired are the service-level guards
	// after the DTO (createFeedback, bug-reports.service.ts:127-132). They fire
	// on a whitespace-only value the DTO @MinLength(1) lets through. Same
	// strings as the bugreports module's create path (kept package-local per
	// package-by-feature: each module owns its sentinels).
	ErrDescriptionRequired = errors.New("description is required")
	ErrSourceRequired      = errors.New("source is required")
	// ErrTitleRequired fires when kind='feature' and the trimmed title is not
	// 1-80 chars (bug-reports.service.ts:140).
	ErrTitleRequired = errors.New("title is required for a feature request (1-80 characters)")
	// ErrBadTargetPlatform fires when kind='feature' and target_platform is
	// absent or not in TargetPlatforms (bug-reports.service.ts:143-145).
	ErrBadTargetPlatform = errors.New("target_platform must be one of: playground, mobile, windows, mac, linux")
	// ErrFeatureNotFound fires when a vote targets a missing / non-feature row
	// (bug-reports.service.ts:174).
	ErrFeatureNotFound = errors.New("feature request not found")
)

// PlatformValid reports whether p is one of the five target platforms.
func PlatformValid(p string) bool {
	for _, k := range TargetPlatforms {
		if k == p {
			return true
		}
	}
	return false
}

// FeatureRow is one row in the GET /feedback board feed. JSON keys + order
// mirror the raw TypeORM BugReport entity exactly (snake_case columns), with
// viewerHasVoted appended LAST (camelCase) the way listPublicFeatures'
// Object.assign(r, { viewerHasVoted }) emits it.
//
// display_no is a TypeORM bigint → serialized as a STRING in JS, so it is typed
// string here. screenshot_path / screenshot_filename appear (the board selects
// every column) even though feature rows never carry a screenshot.
type FeatureRow struct {
	ID                 string  `json:"id"`
	DisplayNo          string  `json:"display_no"`
	Source             string  `json:"source"`
	Description        string  `json:"description"`
	ScreenshotPath     *string `json:"screenshot_path"`
	ScreenshotFilename *string `json:"screenshot_filename"`
	AppVersion         *string `json:"app_version"`
	OsInfo             *string `json:"os_info"`
	UserID             *string `json:"user_id"`
	UserEmail          *string `json:"user_email"`
	Context            any     `json:"context"`
	Status             *string `json:"status"`
	Kind               string  `json:"kind"`
	Title              *string `json:"title"`
	TargetPlatform     *string `json:"target_platform"`
	VoteCount          int32   `json:"vote_count"`
	IsPublic           bool    `json:"is_public"`
	AdminNotes         *string `json:"admin_notes"`
	AiFixProposal      *string `json:"ai_fix_proposal"`
	AiFixProposedAt    *string `json:"ai_fix_proposed_at"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	ViewerHasVoted     bool    `json:"viewerHasVoted"`
}

// Created is the InsertFeedback projection the handler turns into a ref. Kind
// rides along so the handler picks "BUG-"/"FR-" for the display number.
type Created struct {
	ID        string
	DisplayNo int64
	Kind      string
}

// VoteResult is the toggleVote response: { vote_count, hasVoted }.
type VoteResult struct {
	VoteCount int  `json:"vote_count"`
	HasVoted  bool `json:"hasVoted"`
}

// ListResult is the listPublicFeatures response: { rows, total }.
type ListResult struct {
	Rows  []FeatureRow `json:"rows"`
	Total int64        `json:"total"`
}
