package domain

import "strings"

// MaxInputChars caps the rewrite input. Matches the NestJS DTO
// @MaxLength(5000) so a request rejected by one backend is rejected
// by both — no surprises if traffic moves between them mid-rollout.
const MaxInputChars = 5000

// RewriteRequest is a validated value object. Private fields + a
// constructor that returns an error force every caller through the
// same validation path — invalid inputs can never reach the use case
// layer (Rule #1: invariants enforced at the boundary, not scattered).
type RewriteRequest struct {
	text string
	tone Tone
	lang string
}

// NewRewriteRequest validates + builds. Trims whitespace from text to
// match NestJS DTO behaviour. lang is optional (only used by the
// `translate` tone); validated lightly.
func NewRewriteRequest(text, toneStr, lang string) (RewriteRequest, error) {
	t := strings.TrimSpace(text)
	if t == "" {
		return RewriteRequest{}, ErrInvalidInput
	}
	if len(t) > MaxInputChars {
		return RewriteRequest{}, ErrInvalidInput
	}
	tone, err := ParseTone(toneStr)
	if err != nil {
		return RewriteRequest{}, err
	}
	if tone == ToneTranslate && strings.TrimSpace(lang) == "" {
		// translate without a target language is meaningless — better
		// to 400 here than to silently send the request upstream and
		// let OpenAI invent a destination.
		return RewriteRequest{}, ErrInvalidInput
	}
	return RewriteRequest{text: t, tone: tone, lang: strings.TrimSpace(lang)}, nil
}

// Getters — value type, no mutation possible from outside.
func (r RewriteRequest) Text() string { return r.text }
func (r RewriteRequest) Tone() Tone   { return r.tone }
func (r RewriteRequest) Lang() string { return r.lang }

// InputLength is the character count the use case logs into usage_logs
// (NestJS calls it `input_length`).
func (r RewriteRequest) InputLength() int32 { return int32(len(r.text)) }
