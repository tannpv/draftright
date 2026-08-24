// Package parity holds the canonical rewrite-engine catalogs that must stay
// byte-identical to the NestJS backend (the parity authority).
package parity

// ToneMeta mirrors Node's ToneMeta (backend/src/rewrite/tones.ts). The field
// order pins the marshaled JSON key order: id, label, icon, kind.
//
// kind tells a client how to treat the response:
//   - rewrite:   returns { rewritten_text }
//   - grammar:   returns { grammar: { score, issues[] } }
//   - translate: returns { rewritten_text }, requires a target language
type ToneMeta struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Kind  string `json:"kind"`
}

// Tone ids — the single source for the tone-id strings. The Tones catalog, the
// TonePrompts registry keys (prompts.go), and the resolveBasePrompt comparisons
// all reference these, so a tone id lives in exactly one place (#204 finding #6).
const (
	ToneSimple       = "simple"
	ToneNatural      = "natural"
	TonePolished     = "polished"
	ToneConcise      = "concise"
	ToneTechnical    = "technical"
	ToneClaude       = "claude"
	ToneGrammarCheck = "grammar_check"
	ToneTranslate    = "translate"
)

// Tones is the canonical tone catalog — the single source of truth for the set
// of tones the rewrite engine supports. Both DTO validation and the public
// GET /rewrite/tones endpoint derive from this list.
var Tones = []ToneMeta{
	{ID: ToneSimple, Label: "Simple", Icon: "✎", Kind: "rewrite"},
	{ID: ToneNatural, Label: "Natural", Icon: "💬", Kind: "rewrite"},
	{ID: TonePolished, Label: "Polished", Icon: "✨", Kind: "rewrite"},
	{ID: ToneConcise, Label: "Concise", Icon: "⊖", Kind: "rewrite"},
	{ID: ToneTechnical, Label: "Technical", Icon: "🔧", Kind: "rewrite"},
	{ID: ToneClaude, Label: "Claude", Icon: "✦", Kind: "rewrite"},
	{ID: ToneGrammarCheck, Label: "Grammar Check", Icon: "✓", Kind: "grammar"},
	{ID: ToneTranslate, Label: "Translate", Icon: "🌐", Kind: "translate"},
}

// ToneIDs is all valid tone ids, derived from the catalog (used for DTO
// validation). Mirrors Node's TONE_IDS = TONES.map((t) => t.id).
var ToneIDs = func() []string {
	ids := make([]string, len(Tones))
	for i, t := range Tones {
		ids[i] = t.ID
	}
	return ids
}()

// InputKindIDs is all valid input_kind values for POST /rewrite (used for DTO
// validation). Mirrors Node's INPUT_KIND_IDS = ['typed', 'speech']
// (backend/src/rewrite/tones.ts).
var InputKindIDs = []string{"typed", "speech"}
