// Package extraction ports the NestJS extraction module (POST /extract):
// it runs the configured default AI provider over a short message, then
// validates / dedupes the LLM's JSON entity array. Byte-identical port of
// extraction.service.ts (extract → stripCodeFences → validateEntity →
// dedupe → estimateTokens, with the hardcoded system prompt).
//
// JS string semantics: Node measures offsets/lengths in UTF-16 code units
// (String.length / .indexOf), so start/end in the response are UTF-16
// offsets. lenUTF16 / indexUTF16 below match that exactly.
package extraction

import "unicode/utf16"

// EntityKind mirrors the Node EntityKind enum (dto/extract.dto.ts).
type EntityKind string

const (
	KindPhone       EntityKind = "phone"
	KindEmail       EntityKind = "email"
	KindURL         EntityKind = "url"
	KindOTP         EntityKind = "otp"
	KindCreditCard  EntityKind = "creditCard"
	KindAddress     EntityKind = "address"
	KindPersonName  EntityKind = "personName"
	KindDateTime    EntityKind = "dateTime"
	KindBankAccount EntityKind = "bankAccount"
)

// allKinds preserves declaration order, matching Object.values(EntityKind)
// in Node's validateEntity membership check.
var allKinds = []EntityKind{
	KindPhone, KindEmail, KindURL, KindOTP, KindCreditCard,
	KindAddress, KindPersonName, KindDateTime, KindBankAccount,
}

// kindValid reports whether k is a known EntityKind (Node:
// Object.values(EntityKind).includes(kind)).
func kindValid(k EntityKind) bool {
	for _, v := range allKinds {
		if v == k {
			return true
		}
	}
	return false
}

// regexHandledOrder mirrors Node REGEX_HANDLED — kinds the client resolves
// by regex, dropped from LLM output. Slice (not map) so buildSystemPrompt
// emits insertion order phone|email|url|otp|creditCard, matching the JS
// Set iteration order ([...REGEX_HANDLED]).
var regexHandledOrder = []EntityKind{KindPhone, KindEmail, KindURL, KindOTP, KindCreditCard}

// regexHandled is the membership view of regexHandledOrder.
var regexHandled = func() map[EntityKind]bool {
	m := make(map[EntityKind]bool, len(regexHandledOrder))
	for _, k := range regexHandledOrder {
		m[k] = true
	}
	return m
}()

// defaultKinds mirrors the Node default when kinds is absent:
// [Address, PersonName, DateTime, BankAccount].
var defaultKinds = []EntityKind{KindAddress, KindPersonName, KindDateTime, KindBankAccount}

// Entity JSON key order matches Node ExtractedEntityDto:
// kind,value,display,start,end,confidence,meta?.
type Entity struct {
	Kind       EntityKind        `json:"kind"`
	Value      string            `json:"value"`
	Display    string            `json:"display"`
	Start      int               `json:"start"`
	End        int               `json:"end"`
	Confidence float64           `json:"confidence"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// Response key order matches ExtractResponseDto: entities,provider,tokensUsed.
type Response struct {
	Entities   []Entity `json:"entities"`
	Provider   string   `json:"provider"`
	TokensUsed int      `json:"tokensUsed"`
}

// lenUTF16 returns s length in UTF-16 code units, mirroring JS String.length
// (NOT byte or rune length).
func lenUTF16(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// indexUTF16 returns the UTF-16 code-unit offset of the first occurrence of
// substr in s, or -1, mirroring JS String.prototype.indexOf. Go's
// strings.Index returns a BYTE offset, so we convert through the UTF-16
// encoding to match Node's offsets for non-ASCII (e.g. Vietnamese, emoji).
func indexUTF16(s, substr string) int {
	su := utf16.Encode([]rune(s))
	bu := utf16.Encode([]rune(substr))
	if len(bu) == 0 {
		return 0 // JS "".indexOf("") === 0 and any.indexOf("") === 0
	}
	for i := 0; i+len(bu) <= len(su); i++ {
		match := true
		for j := 0; j < len(bu); j++ {
			if su[i+j] != bu[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
