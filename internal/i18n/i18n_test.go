package i18n

import "testing"

func TestCatalogContainsAllCurrentLanguages(t *testing.T) {
	for _, code := range []string{"de", "en", "es"} {
		if !HasLanguage(code) {
			t.Fatalf("language %q missing", code)
		}
		values := Translations(code)
		if values["save"] == "" {
			t.Errorf("language %q has no save translation", code)
		}
		if LanguageLabel(code) == "" {
			t.Errorf("language %q has no selector label", code)
		}
	}
}

func TestUnknownLanguageFallsBackToGerman(t *testing.T) {
	if Translations("xx")["save"] != Translations("de")["save"] {
		t.Fatal("unknown language did not fall back to German")
	}
}
