package bugreports

import (
	"strconv"
	"strings"
	"unicode/utf16"
)

// fields carries the raw multipart text fields, in DTO declaration order
// (CreateBugReportDto): description, source, app_version, os_info,
// user_email, context, website. `present` records whether the multipart
// form supplied the field at all — needed because @IsString on a missing
// (undefined) required field yields a distinct "must be a string" message,
// whereas an empty string does not.
type fields struct {
	description        string
	descriptionPresent bool
	source             string
	sourcePresent      bool
	appVersion         string
	osInfo             string
	userEmail          string
	context            string
	contextPresent     bool
	website            string
}

// lenUTF16 returns the length of s in UTF-16 code units, mirroring JS
// String.length (what class-validator's @MaxLength/@MinLength measure), NOT
// byte or rune length. Package-local copy of the errreport helper (the
// CLAUDE.md guardrail: keep these per-package, do not promote to shared).
func lenUTF16(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// validateBugReport reproduces the NestJS ValidationPipe (whitelist +
// transform) over CreateBugReportDto, with the AllExceptionsFilter
// humanizeValidation rewrite applied to each constraint message. Messages
// are collected per-property in DTO DECLARATION ORDER and joined with ". ".
// Empty result = valid. Code invalid-input, status 400.
//
// Per-property constraints (create-bug-report.dto.ts):
//
//	description  @IsString @MinLength(1)            (required)
//	source       @IsString @MaxLength(50)           (required)
//	app_version? @IsOptional @IsString @MaxLength(50)
//	os_info?     @IsOptional @IsString @MaxLength(255)
//	user_email?  @IsOptional @IsString @MaxLength(255)
//	context?     @IsOptional @IsString              (no length cap)
//	website?     @IsOptional @IsString @MaxLength(255)
//
// Optional fields are strings off a multipart form, so @IsString never trips
// for them; only their @MaxLength can. Required fields run @IsString first
// (fires on a missing/undefined value) then @MinLength/@MaxLength.
func validateBugReport(f fields) string {
	var msgs []string

	// description: @IsString @MinLength(1)
	if !f.descriptionPresent {
		// Undefined → @IsString fires, then @MinLength fires too (a
		// non-string also fails minLength). class-validator emits both,
		// in constraint order: isString, then minLength.
		msgs = append(msgs, "description must be a string")
		msgs = append(msgs, humanizeMinLength("description", 1))
	} else if lenUTF16(f.description) < 1 {
		msgs = append(msgs, humanizeMinLength("description", 1))
	}

	// source: @IsString @MaxLength(50)
	if !f.sourcePresent {
		msgs = append(msgs, "source must be a string")
	} else if lenUTF16(f.source) > 50 {
		msgs = append(msgs, maxLenMsg("source", 50))
	}

	// app_version?: @MaxLength(50)
	if lenUTF16(f.appVersion) > 50 {
		msgs = append(msgs, maxLenMsg("app_version", 50))
	}
	// os_info?: @MaxLength(255)
	if lenUTF16(f.osInfo) > 255 {
		msgs = append(msgs, maxLenMsg("os_info", 255))
	}
	// user_email?: @MaxLength(255)
	if lenUTF16(f.userEmail) > 255 {
		msgs = append(msgs, maxLenMsg("user_email", 255))
	}
	// context?: @IsString only, no length cap → nothing to check.
	// website?: @MaxLength(255)
	if lenUTF16(f.website) > 255 {
		msgs = append(msgs, maxLenMsg("website", 255))
	}

	return strings.Join(msgs, ". ")
}

// maxLenMsg is class-validator's default @MaxLength message, which matches no
// humanizeValidation rewrite pattern and so passes through verbatim.
func maxLenMsg(field string, n int) string {
	return field + " must be shorter than or equal to " + strconv.Itoa(n) + " characters"
}

// humanizeMinLength applies AllExceptionsFilter.humanizeValidation to the
// class-validator @MinLength message "<field> must be longer than or equal to
// <n> characters". That branch matches "must be longer than or equal to" and
// rewrites it to "<TitleCase> must be at least <n> characters." — so the
// MinLength reason is the HUMANIZED string, unlike @MaxLength.
func humanizeMinLength(field string, n int) string {
	title := strings.ToUpper(field[:1]) + field[1:]
	return title + " must be at least " + strconv.Itoa(n) + " characters."
}
