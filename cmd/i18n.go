package cmd

import (
	"os"
	"strings"

	"github.com/Anastylosis/FSS/internal/config"
)

// langFromArgs pre-scans args for --lang before cobra parses anything. It
// stops at "--", handles "--lang ko" and "--lang=ko", and returns "" for a
// missing value or a value starting with "-" (e.g. "--lang --debug" must not
// request a language called "--debug"). Deliberately not a full parser —
// worst case is help in the wrong language.
func langFromArgs(args []string) string {
	for i, a := range args {
		if a == "--" {
			return ""
		}
		if v, ok := strings.CutPrefix(a, "--lang="); ok {
			return v
		}
		if a == "--lang" {
			if i+1 >= len(args) {
				return ""
			}
			v := args[i+1]
			if strings.HasPrefix(v, "-") {
				return ""
			}
			return v
		}
	}
	return ""
}

// resolveLanguage returns the first non-empty language request and whether it
// came from an explicit source. Explicit sources (--lang, FSS_LANG, config
// `language:`) earn a warning when unknown; ambient locale never warns.
func resolveLanguage() (tag string, explicit bool) {
	if v := langFromArgs(os.Args[1:]); v != "" {
		return v, true
	}
	if v := os.Getenv("FSS_LANG"); v != "" {
		return v, true
	}
	if v := config.LanguagePref(); v != "" {
		return v, true
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v, false
		}
	}
	return "", false
}
