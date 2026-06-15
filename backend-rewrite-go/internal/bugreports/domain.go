// Package bugreports ingests user-submitted bug reports (POST /bug-reports,
// multipart/form-data with an optional screenshot). Byte-identical port of
// the NestJS bug-reports module create path. Admin read/list, feedback
// board, voting, and the AI fix-proposal cron are out of scope here.
package bugreports

// Created is the InsertBugReport projection the handler stamps on the row.
// display_no is a bigint sequence in Postgres → int64.
type Created struct {
	ID        string
	DisplayNo int64
}
