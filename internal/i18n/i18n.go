// Package i18n loads all user-facing translations from one embedded JSON file.
package i18n

import (
	"embed"
	"encoding/json"
	"sort"
)

//go:embed translations.json
var translationFS embed.FS

var catalog map[string]map[string]string

func init() {
	data, err := translationFS.ReadFile("translations.json")
	if err != nil {
		panic("i18n: cannot read embedded translations: " + err.Error())
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		panic("i18n: invalid translations.json: " + err.Error())
	}
}

// Translations returns a language catalog with missing entries filled from German.
func Translations(language string) map[string]string {
	selected, ok := catalog[language]
	if !ok {
		selected = catalog["de"]
	}
	result := make(map[string]string, len(catalog["de"]))
	for key, value := range catalog["de"] {
		result[key] = value
	}
	for key, value := range selected {
		result[key] = value
	}
	return result
}

// HasLanguage reports whether a language is present in the central catalog.
func HasLanguage(language string) bool { _, ok := catalog[language]; return ok }

// Languages returns the available language codes in stable alphabetical order.
func Languages() []string {
	result := make([]string, 0, len(catalog))
	for code := range catalog {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}

// LanguageLabel returns the self-name shown in the language selector.
func LanguageLabel(language string) string { return Translations(language)["languageLabel"] }
