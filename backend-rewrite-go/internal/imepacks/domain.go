// Package imepacks serves the static language-pack catalog
// (GET /ime-packs/manifest). It is a byte-identical port of the NestJS
// ime-packs module: a frozen in-memory catalog, no DB, no I/O.
package imepacks

// LanguagePack is a downloadable pack descriptor. Mirrors the Node
// LanguagePack DTO. All fields always present.
type LanguagePack struct {
	URL              string `json:"url"`
	Version          int    `json:"version"`
	SizeBytes        int    `json:"sizeBytes"`
	SHA256           string `json:"sha256"`
	MinEngineVersion int    `json:"minEngineVersion"`
}

// LanguageModule is one catalog entry. pack/wordlistPack are pointers so
// they serialize as omitted (Node drops `undefined`) when nil.
type LanguageModule struct {
	ID           string        `json:"id"`
	DisplayName  string        `json:"displayName"`
	InputMethod  string        `json:"inputMethod"` // composition | candidate | passthrough
	Engine       string        `json:"engine"`      // composition | rime | dictionary | none
	Layout       string        `json:"layout"`      // qwerty | romaji | pinyin
	Bundled      bool          `json:"bundled"`
	Pack         *LanguagePack `json:"pack,omitempty"`
	WordlistPack *LanguagePack `json:"wordlistPack,omitempty"`
}
