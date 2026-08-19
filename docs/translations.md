# Translations

`fss` can show its **help text** in languages other than English. It's a
best-effort, community thing: there is no translation team, no string freeze,
and no expectation that any language stays complete. Anything untranslated
falls back to English string by string — so a half-finished translation is a
perfectly good contribution.

**Shipped**: `ko` (Korean) — machine-assisted, **not yet reviewed by a native
speaker**. Corrections very welcome.

## Using a language

Three explicit sources, highest first, then the ambient desktop locale:

| Source | Example |
|---|---|
| `--lang` flag | `fss --lang ko scrape --help` |
| `FSS_LANG` env var | `FSS_LANG=ko fss stash --help` |
| config `language:` key | `language: ko` in `fss/config.yaml` |
| `LC_ALL` / `LC_MESSAGES` / `LANG` | `LANG=ko_KR.UTF-8 fss --help` |

The **first non-empty** source wins outright — not the first that resolves. So
`FSS_LANG=fr` gives English even if `LANG=ko_KR.UTF-8` is set, because `fr`
was requested and there is no `fr.json`. Anything else would make precedence
meaningless.

Tags are matched loosely: the codeset and modifier are dropped
(`ko_KR.UTF-8` → `ko_KR`), `-` becomes `_`, and a region falls back to the
base language (`ko_KR` → `ko`).

An **explicit** source naming a language with no catalog prints a one-line
warning to stderr and renders English. The ambient locale never warns — a
French desktop user who has never heard of fss's language support must not
see noise on every command.

## What is and isn't translated

**Translated**: command descriptions (`Short`/`Long`), flag help, and cobra's
own help chrome (`Usage:`, `Flags:`, `Available Commands:`, the error prefix).

**Not translated**: runtime output, error messages, `docs/`, and
`fss --version` (scripts parse it). Command names, flag names, and the
metavariables in a usage line (`<studio-url>`, `[command]`) are never
translated — cobra resolves user input against the command name, so
translating it would rename the command.

Shell-completion descriptions come from each command's `Short` and are served
at completion time by the live `fss __complete` process, so they follow your
language setting. That's a feature, not a bug.

## Adding a language

1. Copy `internal/i18n/locales/_template.json` to
   `internal/i18n/locales/<code>.json`, where `<code>` is a lowercase language
   code (`ja`, `de`, `pt_BR`).
2. Fill in what you're confident about. **Leave the rest as `""`** — an empty
   value falls back to English, exactly like a missing key.
3. Run `make test`.
4. Open a PR.

You need no Go knowledge: the keys *are* the English strings, and the file is
flat JSON.

### Rules

- **Never edit a key.** The lookup is an exact string match, so a changed key
  simply stops applying.
- **Keep `%s` / `%d` placeholders identical** to the key's, in the same
  multiset. A mismatch renders as `%!s(MISSING)` in a user's help.
- **No backticks.** pflag reads a backquoted word in a flag usage as the
  value placeholder name.
- **No `{{` or `}}`.** They would be parsed as Go template delimiters.
- Keep command names, flag names, paths, and env var names in ASCII.

Four tests enforce these (`internal/i18n/catalog_test.go`), and
`cmd/i18n_localize_test.go` renders every command in every shipped language,
so breakage surfaces at PR time rather than on a user's `--help`.

## When English changes

Rewording an English string changes the key, which orphans the old
translation. `TestCatalogKeysAreSubsetOfTemplate` catches that — the exact
failure mode where a translation silently stops applying. The fix:

```sh
make i18n-extract     # regenerates _template.json and _pseudo.json
```

then move each affected value onto the new key and commit both.

`_pseudo.json` is a generated catalog whose every value is `«the key»`. It is
not a language; it exists so the test suite can prove the localizer reaches
every string, including the chrome.

## Why only help text

The help strings are assembled at package `init()` time, long before any flag
is parsed, so `fss` walks the finished command tree just before `Execute()` and
rewrites it. That keeps the change to one file plus a small package, and leaves
English output byte-identical to what it was before any of this existed.

The lookup itself, `i18n.T(string) string`, has no notion of "help" — extending
to runtime strings later is wrapping a call site. The first thing that would
need handling is `proceedAfterCollapse` in `cmd/scrape.go`, which parses a
hardcoded `y`/`yes` from the operator; a translated prompt with an untranslated
answer parser is a bug, so that prompt stays English until both move together.
