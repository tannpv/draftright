package imepacks

const packBase = "https://draftright.info/ime-packs"

func wordlist(stem string) *LanguagePack {
	return &LanguagePack{URL: packBase + "/" + stem, Version: 1, SizeBytes: 0, SHA256: "", MinEngineVersion: 1}
}

// Catalog returns the frozen language-module catalog in declared order.
// Byte-identical to the NestJS ime-packs `modules` array.
func Catalog() []LanguageModule {
	return []LanguageModule{
		{ID: "en", DisplayName: "English", InputMethod: "passthrough", Engine: "none", Layout: "qwerty", Bundled: true,
			WordlistPack: wordlist("draftright-wordlist-en-v1.tsv")},
		{ID: "vi", DisplayName: "Tiếng Việt", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true,
			WordlistPack: wordlist("draftright-wordlist-vi-v1.tsv")},
		{ID: "fr", DisplayName: "Français", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true,
			WordlistPack: wordlist("draftright-wordlist-fr-v1.tsv")},
		{ID: "es", DisplayName: "Español", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "de", DisplayName: "Deutsch", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "it", DisplayName: "Italiano", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "pt", DisplayName: "Português", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "ko", DisplayName: "한국어", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "ja", DisplayName: "日本語", InputMethod: "candidate", Engine: "dictionary", Layout: "romaji", Bundled: false,
			Pack: &LanguagePack{URL: packBase + "/draftright-ime-ja-v3.pack", Version: 3, SizeBytes: 2016095,
				SHA256: "100584d329fa2bbe67d9764ee802b7548a12af9ead01e5e50c599281eaf05282", MinEngineVersion: 1}},
		{ID: "zh", DisplayName: "中文", InputMethod: "candidate", Engine: "dictionary", Layout: "pinyin", Bundled: false,
			Pack: &LanguagePack{URL: packBase + "/draftright-ime-zh-v1.pack", Version: 1, SizeBytes: 1907323,
				SHA256: "9cc23ff9c85a76e4d38f5991ffd6e0e23e19eceb702d43a39d3e81a562b98b70", MinEngineVersion: 1}},
	}
}
