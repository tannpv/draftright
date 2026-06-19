// Package inbox is the admin triage aggregator: it merges the error_reports
// and bug_reports admin lists into one time-sorted feed (GET /admin/inbox) and
// serves the lightweight badge counts (GET /admin/inbox/counts). It owns no
// storage — it composes the errreport + bugreports admin use cases through
// small consumer ports, mirroring src/admin/admin.controller.ts inboxCounts()
// + listInbox().
package inbox

// Counts is the GET /admin/inbox/counts body. JSON key order matches the Node
// object literal: { new_bugs, new_features, new_errors, total }.
type Counts struct {
	NewBugs     int `json:"new_bugs"`
	NewFeatures int `json:"new_features"`
	NewErrors   int `json:"new_errors"`
	Total       int `json:"total"`
}

// ErrorItem is an inbox row sourced from error_reports. The JSON key set + order
// mirror the Node errorItems object literal EXACTLY — there are no user_email /
// has_screenshot keys (those belong only to bug items), so the marshalled bytes
// differ per source the same way JSON.stringify omits absent optional keys.
//
//	kind, id, title, platform, app_version, status, created_at,
//	ai_fix_proposal, error_type, severity, occurrence_count
type ErrorItem struct {
	Kind            string  `json:"kind"`
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Platform        *string `json:"platform"`
	AppVersion      *string `json:"app_version"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
	AiFixProposal   *string `json:"ai_fix_proposal"`
	ErrorType       *string `json:"error_type"`
	Severity        *string `json:"severity"`
	OccurrenceCount int     `json:"occurrence_count"`
}

// BugItem is an inbox row sourced from bug_reports. Key set + order mirror the
// Node bugItems object literal EXACTLY — no error_type / severity /
// occurrence_count keys.
//
//	kind, id, title, platform, app_version, status, created_at,
//	ai_fix_proposal, user_email, has_screenshot
type BugItem struct {
	Kind          string  `json:"kind"`
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Platform      *string `json:"platform"`
	AppVersion    *string `json:"app_version"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	AiFixProposal *string `json:"ai_fix_proposal"`
	UserEmail     *string `json:"user_email"`
	HasScreenshot bool    `json:"has_screenshot"`
}

// FeedCounts is the nested counts object in the feed body: { errors, bugs,
// returned }.
type FeedCounts struct {
	Errors   int `json:"errors"`
	Bugs     int `json:"bugs"`
	Returned int `json:"returned"`
}

// Feed is the GET /admin/inbox body: { items, counts }. items is a
// heterogeneous list of ErrorItem / BugItem (JSON-marshalled by their own key
// sets), held as []any so each element keeps its source-specific shape.
type Feed struct {
	Items  []any      `json:"items"`
	Counts FeedCounts `json:"counts"`
}

// errorStatusLabel mirrors AdminController.errorStatusLabel: the int-enum →
// label map for error_reports.status, with index 2 AND 4 both 'resolved', and
// any out-of-range value defaulting to 'new'.
func errorStatusLabel(s int) string {
	labels := []string{"new", "reviewing", "resolved", "fix_proposed", "resolved", "wont_fix"}
	if s < 0 || s >= len(labels) {
		return "new"
	}
	return labels[s]
}
