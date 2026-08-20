package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Anastylosis/FSS/internal/i18n"
	"github.com/Anastylosis/FSS/internal/i18n/cobrai18n"
)

// TestI18nTemplateInSync is the extractor: it regenerates
// internal/i18n/locales/_template.json and _pseudo.json from the live
// command tree and fails (after writing) if they're out of date. Run it via
// `make i18n-extract` and commit the result.
func TestI18nTemplateInSync(t *testing.T) {
	i18n.SetLanguage(i18n.SourceLanguage)
	t.Cleanup(func() { i18n.SetLanguage(i18n.SourceLanguage) })

	keys := cobrai18n.Keys(rootCmd)

	// The template's values are the English source strings, not "". It is
	// never loaded as a catalog (the "_" prefix keeps it out of Available),
	// so this is purely for the humans and the translation platforms reading
	// it: both want a source file that shows the string being translated.
	template := map[string]string{}
	pseudo := map[string]string{}
	for _, k := range keys {
		template[k] = k
		pseudo[k] = "«" + k + "»"
	}

	writeIfChanged(t, "../internal/i18n/locales/_template.json", marshal(t, template))
	writeIfChanged(t, "../internal/i18n/locales/_pseudo.json", marshal(t, pseudo))
}

func marshal(t *testing.T, m map[string]string) []byte {
	t.Helper()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return append(b, '\n')
}

func writeIfChanged(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err == nil && normalizeCRLF(got) == normalizeCRLF(want) {
		return
	}
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	t.Errorf("locales regenerated; commit the changes and re-run")
}

func normalizeCRLF(b []byte) string {
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}
