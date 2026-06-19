package errreport

import (
	"strconv"
	"strings"
)

// maxLen mirrors the @MaxLength constraints on CreateErrorReportDto, in DTO
// declaration order. message/stack_trace have no MaxLength; context is
// @IsObject (skipped) — see backend/src/errors/dto/create-error-report.dto.ts.
type maxLenField struct {
	name string
	val  func(requestBody) string
	max  int
}

var maxLenFields = []maxLenField{
	{"platform", func(b requestBody) string { return b.Platform }, 20},
	{"app_version", func(b requestBody) string { return b.AppVersion }, 50},
	{"severity", func(b requestBody) string { return b.Severity }, 20},
	{"error_type", func(b requestBody) string { return b.ErrorType }, 200},
	{"device_id", func(b requestBody) string { return b.DeviceID }, 100},
	{"website", func(b requestBody) string { return b.Website }, 255},
}

// validateErrorReport reproduces the NestJS ValidationPipe + humanizer for
// CreateErrorReportDto's @MaxLength constraints, in property-declaration
// order. class-validator's default @MaxLength message is exactly
// "<property> must be shorter than or equal to <N> characters", which
// matches no humanizeValidation rewrite pattern, so it passes through
// verbatim. Length is measured in UTF-16 code units (JS String.length).
// Messages join with ". "; empty result = valid. Code invalid-input, 400.
func validateErrorReport(b requestBody) string {
	var msgs []string
	for _, f := range maxLenFields {
		if lenUTF16(f.val(b)) > f.max {
			msgs = append(msgs, f.name+" must be shorter than or equal to "+strconv.Itoa(f.max)+" characters")
		}
	}
	return strings.Join(msgs, ". ")
}
