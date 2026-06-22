package imepacks

import "testing"

// testBase mirrors the production CDN origin so URL assertions below stay
// byte-identical to the shipped manifest.
const testBase = "https://draftright.info/ime-packs"

func TestCatalog_OrderAndCount(t *testing.T) {
	c := Catalog(testBase)
	if len(c) != 10 {
		t.Fatalf("len = %d, want 10", len(c))
	}
	wantIDs := []string{"en", "vi", "fr", "es", "de", "it", "pt", "ko", "ja", "zh"}
	for i, id := range wantIDs {
		if c[i].ID != id {
			t.Errorf("c[%d].ID = %q, want %q", i, c[i].ID, id)
		}
	}
}

func TestCatalog_JaPack(t *testing.T) {
	ja := Catalog(testBase)[8]
	if ja.ID != "ja" || ja.Bundled || ja.Layout != "romaji" || ja.Engine != "dictionary" || ja.InputMethod != "candidate" {
		t.Fatalf("ja header wrong: %+v", ja)
	}
	if ja.Pack == nil {
		t.Fatal("ja.Pack nil")
	}
	want := LanguagePack{
		URL:              "https://draftright.info/ime-packs/draftright-ime-ja-v3.pack",
		Version:          3,
		SizeBytes:        2016095,
		SHA256:           "100584d329fa2bbe67d9764ee802b7548a12af9ead01e5e50c599281eaf05282",
		MinEngineVersion: 1,
	}
	if *ja.Pack != want {
		t.Fatalf("ja.Pack = %+v, want %+v", *ja.Pack, want)
	}
	if ja.WordlistPack != nil {
		t.Fatal("ja.WordlistPack should be nil")
	}
}

func TestCatalog_ZhPack(t *testing.T) {
	zh := Catalog(testBase)[9]
	if zh.Pack == nil || zh.Layout != "pinyin" {
		t.Fatalf("zh wrong: %+v", zh)
	}
	want := LanguagePack{
		URL:              "https://draftright.info/ime-packs/draftright-ime-zh-v1.pack",
		Version:          1,
		SizeBytes:        1907323,
		SHA256:           "9cc23ff9c85a76e4d38f5991ffd6e0e23e19eceb702d43a39d3e81a562b98b70",
		MinEngineVersion: 1,
	}
	if *zh.Pack != want {
		t.Fatalf("zh.Pack = %+v, want %+v", *zh.Pack, want)
	}
}

func TestCatalog_WordlistPacksOnlyEnViFr(t *testing.T) {
	c := Catalog(testBase)
	withWordlist := map[string]bool{"en": true, "vi": true, "fr": true}
	for _, m := range c {
		if withWordlist[m.ID] && m.WordlistPack == nil {
			t.Errorf("%s should have wordlistPack", m.ID)
		}
		if !withWordlist[m.ID] && m.WordlistPack != nil {
			t.Errorf("%s should NOT have wordlistPack", m.ID)
		}
	}
}
