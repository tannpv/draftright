package feedback

import (
	"strconv"
	"strings"
)

// reqBody mirrors the CreateFeedbackDto field names (JSON). present flags record
// whether the key appeared in the body at all — class-validator's @IsString /
// @MinLength fire on an *undefined* required field with a distinct extra message
// that an empty string does not produce, so the handler must tell them apart.
type reqBody struct {
	Kind           string
	Title          string
	TitlePresent   bool
	TargetPlatform string
	PlatformIsSet  bool // target_platform key present AND a JSON string
	Description    string
	DescPresent    bool
	Source         string
	SourcePresent  bool
	AppVersion     string
	OsInfo         string
	UserEmail      string
	Website        string
}

// validateFeedback reproduces the NestJS ValidationPipe (whitelist) over
// CreateFeedbackDto with the AllExceptionsFilter humanizeValidation rewrite,
// collecting per-property messages in DTO DECLARATION ORDER and joining them
// with ". ". Empty result = valid. Code invalid-input, status 400. Lengths are
// UTF-16 code units (JS String.length).
//
// Per-property constraints (create-feedback.dto.ts), declaration order:
//
//	kind            @IsIn(['bug','feature'])                (required, no @IsString)
//	title?          @IsOptional @IsString @MaxLength(80)
//	target_platform?@IsOptional @IsIn(TARGET_PLATFORMS)
//	description     @IsString @MinLength(1) @MaxLength(2000) (required)
//	source          @IsString @MaxLength(50)                 (required)
//	app_version?    @IsOptional @IsString @MaxLength(50)
//	os_info?        @IsOptional @IsString @MaxLength(255)
//	user_email?     @IsOptional @IsString @MaxLength(255)
//	website?        @IsOptional @IsString @MaxLength(255)
//
// Per-field constraint ORDER for a missing required string (verified by running
// class-validator on this DTO): maxLength, minLength, isString. @IsIn and
// @MaxLength messages pass through verbatim (no humanizer rule matches);
// @MinLength is humanized to "<Title> must be at least <n> characters.".
func validateFeedback(b reqBody) string {
	var msgs []string

	// kind: @IsIn — fires when not exactly "bug" or "feature" (incl. missing /
	// non-string). Verbatim class-validator message.
	if b.Kind != "bug" && b.Kind != "feature" {
		msgs = append(msgs, "kind must be one of the following values: bug, feature")
	}

	// title?: @IsString @MaxLength(80). Optional → @IsString only fires when the
	// key is present but not a string; off the JSON we only see a string value,
	// so just the @MaxLength cap can trip.
	if b.TitlePresent && lenUTF16(b.Title) > 80 {
		msgs = append(msgs, maxLenMsg("title", 80))
	}

	// target_platform?: @IsIn(TARGET_PLATFORMS). Optional → only fires when set
	// to a value outside the allow-list. Verbatim @IsIn message.
	if b.PlatformIsSet && !PlatformValid(b.TargetPlatform) {
		msgs = append(msgs, "target_platform must be one of the following values: "+strings.Join(TargetPlatforms, ", "))
	}

	// description: @IsString @MinLength(1) @MaxLength(2000).
	if !b.DescPresent {
		// Undefined → maxLength, minLength, isString all reported, in that
		// order. maxLength does not fire on undefined? class-validator reports
		// it anyway for a missing required string; verified order is
		// max, min, isString. humanize rewrites only the minLength clause.
		msgs = append(msgs, maxLenMsg("description", 2000))
		msgs = append(msgs, humanizeMinLength("description", 1))
		msgs = append(msgs, "description must be a string")
	} else {
		if lenUTF16(b.Description) > 2000 {
			msgs = append(msgs, maxLenMsg("description", 2000))
		}
		if lenUTF16(b.Description) < 1 {
			msgs = append(msgs, humanizeMinLength("description", 1))
		}
	}

	// source: @IsString @MaxLength(50).
	if !b.SourcePresent {
		msgs = append(msgs, maxLenMsg("source", 50))
		msgs = append(msgs, "source must be a string")
	} else if lenUTF16(b.Source) > 50 {
		msgs = append(msgs, maxLenMsg("source", 50))
	}

	// app_version?: @MaxLength(50)
	if lenUTF16(b.AppVersion) > 50 {
		msgs = append(msgs, maxLenMsg("app_version", 50))
	}
	// os_info?: @MaxLength(255)
	if lenUTF16(b.OsInfo) > 255 {
		msgs = append(msgs, maxLenMsg("os_info", 255))
	}
	// user_email?: @MaxLength(255)
	if lenUTF16(b.UserEmail) > 255 {
		msgs = append(msgs, maxLenMsg("user_email", 255))
	}
	// website?: @MaxLength(255)
	if lenUTF16(b.Website) > 255 {
		msgs = append(msgs, maxLenMsg("website", 255))
	}

	return strings.Join(msgs, ". ")
}

// maxLenMsg is class-validator's default @MaxLength message — no humanizeValidation
// rule matches, so it passes through verbatim.
func maxLenMsg(field string, n int) string {
	return field + " must be shorter than or equal to " + strconv.Itoa(n) + " characters"
}

// humanizeMinLength applies AllExceptionsFilter.humanizeValidation to the
// @MinLength message: "<field> must be longer than or equal to <n> characters"
// matches the "must be longer than or equal to" branch and is rewritten to
// "<TitleCase> must be at least <n> characters.".
func humanizeMinLength(field string, n int) string {
	title := strings.ToUpper(field[:1]) + field[1:]
	return title + " must be at least " + strconv.Itoa(n) + " characters."
}
