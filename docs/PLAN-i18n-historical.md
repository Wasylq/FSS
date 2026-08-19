# Translatable help text for fss — implementation plan

> ## STATUS — implemented, kept for reference
>
> All ten steps below shipped. The user-facing documentation is
> [translations.md](translations.md); the load-bearing conventions are in
> `CLAUDE.md`. This file is retained for the cobra-internals research in
> "Verified against cobra v1.10.2" — the line references are to v1.10.2 and
> will drift on upgrade, but `TestUsageTemplateMatchesCobra` and
> `TestCobraDefaultsUnchanged` turn any such drift into a red test.
>
> Deviations from the plan as written, all in step 7:
> - `catalog_test.go` reads every catalog through the embed FS rather than
>   `_template.json` from disk. Same bytes, one less path to get wrong.
> - `TestLocalizeRestoreRoundTrip` snapshots the tree *after* forcing cobra's
>   lazy help command and help/version flags. `Localize` deliberately does not
>   un-materialise those, so a bare-tree baseline compares against a tree that
>   legitimately grew an entry.
> - `TestLocalizeSyntheticTree` stayed as the plan resolved it: mechanics only,
>   with catalog-driven rendering covered on the real tree in `cmd/`.

## Context

fss is picking up Korean traffic — Naver search (`m.search.naver.com` + `search.naver.com`) accounts for 109 of the ~198 GitHub referral views, more than GitHub itself. This is a deliberate, low-commitment nod to those users: make the **CLI help text** translatable, ship a Korean catalog, and open an easy contribution path for anyone who wants to add or fix a language.

Explicitly **not** a full localization project. Runtime output, error messages, and `docs/` stay English. The design keeps the door open — the lookup is a plain `i18n.T(string) string` usable from any package, and the catalog format has no notion of "help" — so extending later is wrapping a call site and re-running one make target.

### The constraint that shapes the design

Cobra help strings (`Short`, `Long`, flag `Usage`) are set at package `init()` time, long before `PersistentPreRunE` runs. So instead of wrapping ~98 strings across the 20 files in `cmd/`, we **walk the assembled command tree just before `rootCmd.Execute()`** and rewrite each string through the catalog. Result: only `cmd/root.go` is modified and two files are added under `cmd/` (`i18n.go` plus tests); the other 19 command files are untouched. English output is byte-identical to today (the walk short-circuits when the language is English).

### Verified against cobra v1.10.2 source (module cache, `command.go` unless noted)

- `UsageTemplate()` / `HelpTemplate()` / `ErrPrefix()` fall back to `c.parent.…` (`:592-651`) → **set once on root, inherits to all commands**.
- `defaultHelpTemplate` (`:2042`) contains **zero English**; only `defaultUsageTemplate` (`:1942-1970`) has chrome — 9 labels, listed verbatim below.
- When no custom template is set, cobra v1.10 renders through `defaultUsageFunc` (a Go function, `:1973`) without parsing any template. Once we `SetUsageTemplate`, rendering switches to the template path — one more reason the English case must not set a template at all.
- `SetUsageTemplate("")` **resets to the default** (`:318-324` sets `c.usageTemplate = nil`); same for `SetHelpTemplate` (`:358`). `SetErrPrefix("")` restores the `"Error:"` fallback (`:643-652`). → a clean restore path exists, used by `Localize`'s restore func below.
- `SetUsageTemplate(s)` stores `tmpl(s)`, which does `template.Must(t.Parse(text))` **inside the render closure** (`cobra.go:179-189`) → a stray `{{` in a contributed catalog panics on `--help`, not at startup. Needs three layers of defence (see Risks).
- `InitDefaultHelpFlag` builds usage by concatenation: `"help for " + c.DisplayName()` (`:1219-1232`), sets the `FlagSetByCobraAnnotation` annotation (`:1230`), and is called **only for the command being executed** → must be called on every node in our walk, and needs a parameterized key. Same pattern for `InitDefaultVersionFlag` (`:1234-1258`, annotation at `:1256`).
- The `help` command's `Long` embeds the root name by concatenation (`InitDefaultHelpCmd`, `:1263-1273`) → second parameterized key.
- `Command.Name()` is `Use` up to the first space, and `Find()` resolves user input against it (`:1541`) → **`Use` must never be translated.**
- `FlagUsages()` passes `cols=0`, so pflag never wraps (`pflag/flag.go:656,777`), and all padding derives from ASCII flag/command names → **no CJK alignment or byte-vs-rune wrapping bug.**

### Verified against this repo

- Module path: `github.com/Anastylosis/FSS`. cobra v1.10.2 (`go.mod:8`); `golang.org/x/text v0.40.0` is already a direct dep (`go.mod:10`) but is **not** to be used here (see below).
- `rootCmd` is **unexported** (`cmd/root.go:15`); all existing `cmd` tests are in `package cmd` and drive `rootCmd.SetArgs(…)` + `rootCmd.Execute()` directly (`cmd/check_test.go:11-22`, `cmd/detect_test.go`, `cmd/scrape_test.go:784`). → every test that needs the real tree lives in `package cmd`; nothing outside `cmd` can (or should) reach `rootCmd`.
- `cmd.Execute()` is `cmd/root.go:51-55`; `main.go:56` calls `cmd.SetVersion(...)` before `cmd.Execute()`, so `rootCmd.Version` is always set by the time we run.
- No command sets `Example` or `Aliases` (verified by grep) — only `Short`, `Long`, and flag usages exist to translate.
- fss ships its own `cmd/completion.go` and `cmd/version.go`. There is no `--config` flag; the config path is fixed XDG (`internal/config/config.go:140`).
- `internal/config/config_test.go:13-26` has `isolateXDG(t)` (Setenv + `xdg.Reload()` + cleanup) — reuse it for the `LanguagePref` tests.
- `Makefile` `help` target extracts targets with awk class `[a-zA-Z_-]+` (`Makefile:34`) — **this does not match a digit**, so the `i18n-extract` target requires widening the class (see Makefile step).
- Precedent for "test that regenerates a file and fails": `TestReadmeScraperCount` / `TestSitesMdInSync` in `internal/scrapers/all/all_test.go:18-51`.

---

## Design

### Package layout

```
internal/i18n/                  # stdlib only — no cobra, so any package can use T()
    i18n.go                     # T, SetLanguage, Language, Available, Normalize
    i18n_test.go                # unit tests for the above
    catalog_test.go             # validation of every shipped locales/*.json
    locales/
        _template.json          # generated; the contract translators work against
        _pseudo.json            # generated; «key» values, for the chrome-coverage test
        ko.json                 # machine-assisted, awaiting native review
internal/i18n/cobrai18n/        # the cobra-aware walk
    cobrai18n.go                # Localize(root) (restore func()), Keys(root)
    cobrai18n_test.go           # SYNTHETIC command trees only — never imports cmd
cmd/i18n.go                     # resolveLanguage, langFromArgs
cmd/i18n_test.go                # langFromArgs + resolveLanguage precedence
cmd/i18n_template_test.go       # TestI18nTemplateInSync (the extractor)
cmd/i18n_localize_test.go       # real-tree tests: render every catalog, pseudo
                                # coverage, name invariants, restore round-trip
```

Two packages so `internal/i18n` stays dependency-free. The public root packages (`scraper/`, `output/`, …) can legally import `internal/` — Go's rule only blocks other modules — so `internal/` is the right home and costs nothing for later extension.

**Test placement rule (this fixes a flaw in the previous draft):** `rootCmd` is unexported, so any test that touches the real command tree MUST be in `package cmd`. `cobrai18n_test.go` stays in `package cobrai18n_test` but builds its own throwaway `*cobra.Command` trees; it never imports `cmd`. Do not export `rootCmd` or add an accessor.

### Catalog format

Flat JSON, **English source string as key**, embedded via `//go:embed locales/*.json`:

```json
{ "scrape all scenes and metadata from a studio URL": "스튜디오 URL에서 …" }
```

- Missing key **or empty value** → English passthrough. A half-finished translation degrades string by string; a stale one (after an English reword) degrades rather than showing wrong text.
- No key registry to maintain, and a translator needs zero Go knowledge.
- Keys containing newlines (multi-line `Long` strings) are fine — JSON escapes them as `\n`.

### `internal/i18n/i18n.go`

```go
package i18n

import ("embed"; "encoding/json"; "sort"; "strings"; "sync/atomic")

//go:embed locales/*.json
var localeFS embed.FS

const SourceLanguage = "en" // no catalog file; T is the identity function for it

// One pointer so tag and catalog swap atomically; nil map means English.
type state struct {
    tag string
    m   map[string]string
}
var active atomic.Pointer[state] // nil pointer == English; lock-free for -race

func T(s string) string {
    st := active.Load()
    if st == nil || st.m == nil { return s }
    if v, ok := st.m[s]; ok && v != "" { return v }
    return s
}

func SetLanguage(tag string) string
func Language() string      // active tag, SourceLanguage if none
func Available() []string   // "en" first, then catalogs sorted; skips "_"-prefixed
func Normalize(tag string) string
```

**`Normalize(tag)` spec** (pure function, table-tested):
1. Trim whitespace.
2. Strip everything from the first `.` or `@` (drops codeset/modifier: `ko_KR.UTF-8` → `ko_KR`).
3. Map `-` → `_` (BCP-47 `ko-KR` → `ko_KR`).
4. If the result is empty, `C`, or `POSIX` (case-insensitive) → return `""`.
5. If the result contains `/`, `\`, or `.`, or exceeds 16 bytes → return `""` (a hostile `$LANG` must not steer an `embed.FS` lookup).
6. Case-fold around the first `_`: lowercase before it, uppercase after (`KO_kr` → `ko_KR`; no `_` → all lowercase).

**`SetLanguage(tag)` spec** — returns the language actually installed:
1. `raw := strings.TrimSpace(tag)`; `norm := Normalize(raw)`.
2. If `norm == ""` or `norm == SourceLanguage` → install English (store `&state{tag: SourceLanguage}`), return `SourceLanguage`.
3. Build a candidate list, deduped, in order: `raw` **only if** it matches `^[A-Za-z0-9_]{1,16}$` (this is what lets tests load `_pseudo`, whose name `Normalize` would mangle — and the regex keeps arbitrary input away from the FS); then `norm`; then `norm` truncated at the first `_` (base language: `ko_KR` → `ko`).
4. For each candidate, try `localeFS.ReadFile("locales/" + c + ".json")` + `json.Unmarshal` into `map[string]string`. First success: install `&state{tag: c, m: m}`, return `c`.
5. All candidates fail → install English, return `SourceLanguage`.

**`Available()`**: `localeFS.ReadDir("locales")`, strip `.json`, skip names starting with `_`, sort, prepend `"en"`.

**Do not use `golang.org/x/text/language`.** Already a direct dep, so no `go.mod` churn — but `Parse` + `NewMatcher` links in CLDR tables to answer a question that "strip codeset, try `ko_KR`, then `ko`" answers exactly as well for 1–5 catalogs.

**Malformed JSON: silent English fallback at runtime, loud in CI.** The file is compiled in, so malformed means a contributor broke it and it got merged — which `TestCatalogsParse` prevents. Crashing a user who set `LANG=ko` is the wrong trade.

### `internal/i18n/cobrai18n/cobrai18n.go`

```go
// Localize rewrites the tree's help strings through the active catalog and
// returns a function that restores every string it changed. Production
// (cmd.Execute) discards the restore func; tests defer it so the shared
// rootCmd singleton is returned to its English state for the next test.
func Localize(root *cobra.Command) (restore func())

// Keys returns every translatable string the tree exposes, including the
// chrome and parameterized keys. Same walk as Localize, so the extractor
// cannot drift from the localizer.
func Keys(root *cobra.Command) []string
```

Both are thin wrappers over one internal `visit(c, fn)` recursion (`fn(c)` then recurse over `c.Commands()`) plus one internal `prepare(root)`.

```go
func Localize(root *cobra.Command) (restore func()) {
    // No-op for English: default output stays byte-identical to today —
    // no template substitution, no flag mutation, no help command
    // materialised earlier than cobra would have done it.
    if i18n.Language() == i18n.SourceLanguage { return func() {} }
    ...
}
```

**`prepare(root)`** — force lazy construction (cobra only builds these for the command it runs):

```go
root.InitDefaultVersionFlag()  // SetVersion already ran — main.go calls it first
root.InitDefaultHelpCmd()      // root only; cobra's later call is a no-op re-add
// then, via visit: c.InitDefaultHelpFlag() for every command
```

All three are idempotent (each checks before adding). Do **not** call `InitDefaultCompletionCmd` — fss ships its own `cmd/completion.go`, so cobra bails at `completions.go:750` anyway. Comment that if fss ever drops it, cobra's replacement won't be localized.

**Per command, translate exactly:** `Short` and `Long` (skip empty). **Never touch:** `Use`, `Example`, `Aliases`, `Deprecated`, annotations.

**Flag traversal**: use neither `LocalFlags()` nor `InheritedFlags()` (both allocate new FlagSets aliasing the *same* `*pflag.Flag`, so mutation would still work but `--debug` would be visited once per command). Instead keep a `seen map[*pflag.Flag]bool` (pointer identity) and `VisitAll` over `c.Flags()` and `c.PersistentFlags()` for every command.

**Per flag:**
- If `len(f.Annotations[cobra.FlagSetByCobraAnnotation]) > 0` (the cobra-generated `help`/`version` flags): parameterized handling. Extract the name from the existing usage (it is exactly `"help for " + name` / `"version for " + name`), or equivalently use `c.DisplayName()` of the command being visited; set `f.Usage = fmt.Sprintf(i18n.T(keyHelpFor), name)` (resp. `keyVersionFor`). If the usage is the nameless variant, use the `"… for this command"` key verbatim.
- Otherwise: `f.Usage = i18n.T(f.Usage)`.

**The `help` command** (found after `prepare` among `root.Commands()` by `Name() == "help"`): `Short` is the literal key `"Help about any command"`; `Long` is rebuilt as `fmt.Sprintf(i18n.T(keyHelpLong), root.DisplayName())`.

**Restore mechanics**: while walking, before each mutation append a closure capturing the old value (`c.Short`, `c.Long`, `f.Usage`) to a slice; `restore` runs them in reverse and finishes with `root.SetUsageTemplate("")` and `root.SetErrPrefix("")` (both verified to reset to cobra defaults). The help command and help/version flags materialised by `prepare` stay materialised with their English strings restored — harmless, since cobra would add identical ones at `Execute()` and the init functions are no-ops when they already exist.

**`Use` is never translated** — locked behind `const translateUseLine = false` with the reasoning in a comment: `Name()` is `Use` up to the first space and `Find()` matches against it, so any edit is one bug from renaming the command; `UseLine()` does `strings.Replace(c.Use, c.Name(), …)` and garbles if they diverge; and `<studio-url>` / `[command]` are shell-level metavariables every major CLI leaves in ASCII.

**Chrome keys** — declare as consts in one block, exactly these (the 9 `defaultUsageTemplate` labels, the error prefix, and 6 parameterized/help-command strings):

```go
const (
    keyUsage      = "Usage:"
    keyAliases    = "Aliases:"
    keyExamples   = "Examples:"
    keyAvailable  = "Available Commands:"
    keyAdditional = "Additional Commands:"
    keyHelpTopics = "Additional help topics:"
    keyFlags      = "Flags:"
    keyGlobal     = "Global Flags:"
    keyUseForMore = `Use "%s [command] --help" for more information about a command.`
    keyErrPrefix  = "Error:"
    keyHelpFor        = "help for %s"
    keyHelpForThis    = "help for this command"
    keyVersionFor     = "version for %s"
    keyVersionForThis = "version for this command"
    keyHelpShort      = "Help about any command"
    keyHelpLong       = "Help provides help for any command in the application.\nSimply type %s help [path to command] for full details."
)
```

(`--lang`'s own usage string is a normal flag usage collected by the walk, like `--debug`'s — not a chrome const.)

**Usage template** — override only the usage template (the help template has no English), on root only:

```go
func usageTemplate() string {
    l := func(s string) string { return safe(i18n.T(s)) }
    more := strings.Replace(l(keyUseForMore), "%s", "{{.CommandPath}}", 1)
    return l(keyUsage) + `{{if .Runnable}}` + … // structure copied byte-for-byte
                                                // from cobra's defaultUsageTemplate,
                                                // with each label routed through l()
}

// safe strips {{ and }} before splicing translated text into template source.
// cobra parses lazily inside the render closure with template.Must, so a stray
// delimiter would panic the process on `fss --help` rather than failing at
// startup.
func safe(s string) string {
    s = strings.ReplaceAll(s, "{{", "")
    return strings.ReplaceAll(s, "}}", "")
}
```

Correctness anchor: with English active (`T` = identity), `usageTemplate()` must be **byte-identical** to `new(cobra.Command).UsageTemplate()` (public method; returns `defaultUsageTemplate` when none is set). That is `TestUsageTemplateMatchesCobra`. Note the template has a `{{.Title}}` branch for command groups — fss uses no groups; keep the branch verbatim, group titles are a non-goal.

Apply with `root.SetUsageTemplate(usageTemplate())` and `root.SetErrPrefix(i18n.T(keyErrPrefix))` — both inherit down the tree.

**`Keys(root)`** returns (dupes fine — the consumer builds a map): all 16 chrome consts, then per command `Short` + `Long` (skip empty), then per unseen flag its `f.Usage` — except cobra-annotated flags, which are already covered by the parameterized consts and must be skipped (their concatenated form like `"help for fss"` must NOT appear as a key).

**Non-goals, documented in a comment, not attempted**: pflag's own messages (`unknown flag: --foo`) are `fmt.Errorf`'d with no hook — `SetFlagErrorFunc` only wraps, doesn't re-render. Same for cobra's `unknown command %q`, `required flag(s) … not set`, `Did you mean this?`. The escape hatch if ever wanted is `SilenceErrors = true` + printing in `cmd.Execute()` — mention it, don't build it. `fss --version` output stays English (scripts parse it).

### Language resolution — `cmd/i18n.go`

```go
// resolveLanguage returns the first non-empty language request and whether it
// came from an explicit source. Explicit sources (--lang, FSS_LANG, config
// `language:`) earn a warning when unknown; ambient locale (LC_ALL,
// LC_MESSAGES, LANG) never warns — a French desktop user who has never heard
// of fss's language support must not see noise on every command.
//
// First NON-EMPTY source wins outright (not the first that resolves):
// FSS_LANG=fr with no fr.json must give English, not fall through to LANG,
// or precedence is meaningless.
func resolveLanguage() (tag string, explicit bool) {
    if v := langFromArgs(os.Args[1:]); v != "" { return v, true }
    if v := os.Getenv("FSS_LANG"); v != "" { return v, true }
    if v := config.LanguagePref(); v != "" { return v, true }
    for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
        if v := os.Getenv(k); v != "" { return v, false }
    }
    return "", false
}
```

`langFromArgs(args []string) string` pre-scans ahead of flag parsing: stops at `--`; handles `--lang ko` and `--lang=ko`; returns `""` for a missing value or a value that starts with `-` (`fss --lang --debug` must not request a language called `--debug`). It is deliberately not a full parser — worst case is help in the wrong language; say so in a comment.

Register **only `--lang`** (one spelling; pflag has no aliases, and `SetNormalizeFunc` is heavier machinery than a niche flag warrants) as a persistent flag on root, in `init()` next to `--debug` (`cmd/root.go:36-38`):

```go
rootCmd.PersistentFlags().String("lang", "", "help language code, e.g. ko (see docs/translations.md)")
```

Its value is never read via `GetString` — it exists so cobra accepts the flag and shows it under `Global Flags:`; the real read is the `langFromArgs` pre-scan. Comment that, or a reader will hunt for the missing consumer.

### Wiring — `cmd/root.go`

Replace `Execute()` (`cmd/root.go:51-55`) with:

```go
// Execute runs the root command and exits non-zero on failure.
func Execute() {
    tag, explicit := resolveLanguage()
    resolved := i18n.SetLanguage(tag)
    if explicit && resolved == i18n.SourceLanguage && i18n.Normalize(tag) != i18n.SourceLanguage {
        fmt.Fprintf(os.Stderr, "unknown language %q; using English (available: %s)\n",
            tag, strings.Join(i18n.Available(), ", "))
    }
    cobrai18n.Localize(rootCmd) // restore func deliberately dropped: one Execute per process
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

In `cmd.Execute()`, **not `main.go`** — and that's a real property, not a coincidence: the existing `cmd` tests drive `rootCmd.Execute()` directly, bypassing `cmd.Execute()`, so a developer with `LANG=ko_KR.UTF-8` gets identical test behaviour to CI. Nothing else in `root.go` changes; do not touch `PersistentPreRunE` or its skip list.

### Config — `internal/config/config.go`

Add the field (**mandatory** — without it `warnUnknownConfigKeys` prints a warning on every command for anyone who sets `language:`):

```go
// Language selects the help-text language, e.g. "ko". Empty means English.
// Overridden by FSS_LANG and --lang. An unrecognised value falls back to
// English rather than failing, so a config written for a newer fss still
// works on an older binary.
Language string `yaml:"language"`
```

Do **not** validate it in `Validate()` (`config.go:211`) — forward compatibility, matching the reasoning already documented above `warnUnknownConfigKeys`.

Add a separate silent reader rather than reusing `config.Load()`, which logs warnings (file-mode at `config.go:154`, unknown-keys at `:205`) and is called *again* from `PersistentPreRunE` — sharing it would double every warning, and memoizing would break the config tests that isolate `XDG_CONFIG_HOME` per case:

```go
// LanguagePref returns the `language:` setting, or "" if there is no config
// file, it cannot be read, or it does not parse. Best-effort by design: a
// broken config must still let `fss --help` work.
func LanguagePref() string {
    path, err := xdg.SearchConfigFile("fss/config.yaml")
    if err != nil { return "" }
    // open; io.LimitReader(f, maxConfigBytes+1); bail to "" if over the cap
    // or on any error; sanitizeWindowsPaths; unmarshal into a private probe
    // struct{ Language string `yaml:"language"` } — NOT Config, so defaults
    // and Validate stay out of the picture.
}
```

Hoist `maxConfigBytes` from the local const inside `Load` (`config.go:158`, currently `1 << 20`) to a package-level const so both readers share it. Cost of the extra read: one stat + one size-capped read + YAML parse, once per invocation.

Add a commented `# language: ko` block to both `config.example.yaml` and `cmd/config_default.yaml`.

### Extraction & sync tests

The extractor is a **test, not a binary** — it needs `rootCmd`, which only `package cmd` has, and it mirrors the regenerate-then-fail precedent in `internal/scrapers/all/all_test.go`.

`cmd/i18n_template_test.go` — `TestI18nTemplateInSync`:
1. `i18n.SetLanguage(i18n.SourceLanguage)` first (snapshot the keys, not whatever the developer's `$LANG` is), `t.Cleanup` the same.
2. `keys := cobrai18n.Keys(rootCmd)`; build `template := map[string]string` with `""` values and `pseudo := map[string]string` with `"«" + key + "»"` values (parameterized keys keep their `%s` inside the guillemets, so `fmt.Sprintf` still works).
3. `json.MarshalIndent` each (map keys sort automatically → deterministic), append `"\n"`.
4. Compare against `../internal/i18n/locales/_template.json` and `_pseudo.json` (paths relative to the `cmd/` package dir), folding CRLF before comparing (same note as `TestSitesMdInSync`).
5. On mismatch: write both files, then `t.Errorf("locales regenerated; commit the changes and re-run")` — auto-write-then-fail, so the change lands in git.

`Makefile`:
```make
i18n-extract: ## Regenerate internal/i18n/locales/_template.json and _pseudo.json from the command tree.
	$(GO) test -count=1 -run TestI18nTemplateInSync ./cmd/
```
**Also** widen the `help` target's awk class at `Makefile:34` from `[a-zA-Z_-]+` to `[a-zA-Z0-9_-]+`, or `make help` silently omits `i18n-extract` (the `1` is a digit). No CI change — `make test` already runs the sync test.

`internal/i18n/catalog_test.go` — iterates every `locales/*.json` via the embed FS (including `_`-prefixed ones), reading `_template.json` from disk for the subset test:
1. `TestCatalogsParse` — every file unmarshals into `map[string]string`.
2. `TestCatalogKeysAreSubsetOfTemplate` — every key of every non-`_` catalog exists in `_template.json`. Catches stale keys after an English reword — the exact failure mode where a translation silently stops applying. Error text must say: ``re-run `make i18n-extract`, then update the catalog``.
3. `TestCatalogPlaceholderParity` — for every **non-empty** value, the multiset of `%`-verbs (`%s`, `%d`, `%q`) equals the key's, or the user's help gets `%!s(MISSING)`.
4. `TestCatalogsHaveNoTemplateDelimiters` — no `{{` or `}}` in any value; `safe()` strips them at runtime, but a translator should hear about it at PR time.

`internal/i18n/cobrai18n/cobrai18n_test.go` — **synthetic trees only** (`package cobrai18n_test`, imports `cobra`, `i18n`, `cobrai18n`; never `cmd`):
- **`TestUsageTemplateMatchesCobra`** — with English active, `usageTemplate()` (exported to the test via a small `export_test.go`, or made a public `UsageTemplate()`) equals `new(cobra.Command).UsageTemplate()` byte-for-byte. Turns a cobra template change on upgrade into a red test.
- **`TestCobraDefaultsUnchanged`** — throwaway commands: after `InitDefaultHelpFlag`, `Flags().Lookup("help").Usage == "help for x"`; after `InitDefaultVersionFlag` on a command with `Version` set, `"version for x"`; after `InitDefaultHelpCmd` on a parent with a child, the help command's `Short == "Help about any command"` and its `Long` equals the concatenated form. Guards the parameterized keys against a silent cobra upgrade.
- **`TestSafeStripsDelimiters`** — table test for `safe`.
- **`TestLocalizeSyntheticTree`** — build a two-level tree with a `Short`, a `Long`, a local flag, and a persistent flag; `i18n.SetLanguage("_pseudo")` won't help here (pseudo keys come from the real tree), so this test installs a catalog by… it can't: `i18n` has no injection API, by design. Instead: assert **mechanics** that don't need a catalog — `Localize` with English active returns a no-op (tree byte-identical, no template set); and `Keys` on the synthetic tree returns exactly the expected key set (chrome consts + the tree's strings, cobra-annotated flags excluded). Catalog-driven rendering is covered on the real tree in `cmd/`.

`cmd/i18n_localize_test.go` — `package cmd`, the real tree. **Every test here follows the hygiene pattern** (this fixes the shared-singleton flaw in the previous draft):

```go
i18n.SetLanguage(<lang>)
restore := cobrai18n.Localize(rootCmd)
t.Cleanup(restore)
t.Cleanup(func() { i18n.SetLanguage(i18n.SourceLanguage) })
```

- **`TestLocalizeRestoreRoundTrip`** — capture every command's `Short`/`Long` and root's `UsageString()` in English; localize to `ko`; `restore()`; assert everything byte-identical to the captures. This is the invariant that makes the shared `rootCmd` safe for every other test in the package.
- **`TestEveryCatalogRendersHelp`** — the highest-value test: for each tag in `i18n.Available()[1:]` **plus `"_pseudo"`**: SetLanguage, Localize, then for every command in the tree call `c.UsageString()` and `c.Help()` (with `rootCmd.SetOut`/`SetErr` pointed at a buffer; reset with `SetOut(nil)`/`SetErr(nil)` in cleanup), restore, next language. Any `{{` breakage panics/errors here instead of on a user's `--help`.
- **`TestPseudoLocaleCoversChrome`** — SetLanguage(`"_pseudo"`), Localize, then assert **presence of pseudo-wrapped markers** (absence-of-English is unprovable since `«Flags:»` contains `Flags:` as a substring): root `UsageString()` contains `«Usage:»`, `«Available Commands:»`, `«Flags:»`, `«help for fss»`, `«Help about any command»`, and `«Use "fss [command] --help"` (prefix); `scrape` `UsageString()` contains `«Global Flags:»` and `«help for scrape»`. Empirically proves the chrome key list is complete and the parameterized re-insertion works. (`Aliases:`/`Examples:`/`Additional…` never render — fss has none; the template test still carries their keys.)
- **`TestCommandNamesUnchangedAfterLocalize`** — before localizing to `ko`, walk the tree recording `(cmd, Use)` pairs and `(*pflag.Flag, Name)` pairs; after `Localize`, assert each recorded `Use` and flag `Name` is byte-identical (pointer-identity lookups; the help command/flags added by `prepare` are new additions and exempt). The single most important invariant.
- **`TestUnknownExplicitLanguageWarns`** (optional but cheap) — drive `Execute`-level behaviour indirectly: assert `resolveLanguage` + the warning condition on a fake env via `t.Setenv("FSS_LANG", "xx")`.

`cmd/i18n_test.go` — pure-function tests:
- `langFromArgs` table: `--lang ko` / `--lang=ko` / `--lang` at end-of-args (→ `""`) / `--lang --debug` (→ `""`) / `-- --lang=ko` (→ `""`) / `scrape --lang ko` (→ `ko`).
- `resolveLanguage` precedence with `t.Setenv`: FSS_LANG beats LC_ALL; LC_ALL beats LANG; `explicit` is true for FSS_LANG and false for LANG; empty env → `("", false)`. (The `--lang` tier reads `os.Args` — cover it by calling `langFromArgs` directly, not by mutating `os.Args`.)

`internal/config/` — extend `unknown_keys_test.go` (`captureLog` style) with `TestLanguageKeyIsKnown` (`language: ko` produces no warning) and add `TestLanguagePref` in `config_test.go` style using `isolateXDG(t)`: no file → `""`; file with `language: ko` → `"ko"`; malformed YAML → `""`.

### Docs & contribution path

**`docs/translations.md`** (new, ~60 lines) — tone is a genuine invitation with no promise attached:

> It's a best-effort, community thing: no translation team, no string freeze, no expectation that any language stays complete. Anything untranslated falls back to English string by string — so a half-finished translation is a perfectly good contribution.
>
> **Adding a language**: copy `_template.json` → `<code>.json` (two-letter lowercase code, e.g. `ja.json`), fill in what you're confident about, **leave the rest as `""`**, `make test`, open a PR.
>
> **Rules**: never edit a key; keep `%s`/`%d` placeholders identical; no backticks (pflag reads a backquoted word as a value placeholder); no `{{` or `}}`; command and flag names are never translated; `<studio-url>`/`[command]` stay English by design.
>
> **Translated**: command descriptions, flag help, cobra's help chrome. **Not**: runtime output, errors, `docs/`, `fss --version`.
>
> **Shipped**: `ko.json` — machine-assisted, **not yet reviewed by a native speaker**. Corrections very welcome.

Also note there: shell-completion descriptions come from `Short` at completion time via the live `fss __complete` process, so they follow the user's language setting — a feature, not a bug.

**`CONTRIBUTING.md`** — a 5-line `## Translations` section after `## Adding a New Scraper`, pointing at the doc. The file is 611 lines of scraper mechanics; don't inflate it.
**`README.md`** — one bullet in the `## Documentation` list, plus `# language: ko` in the config example. No new top-level section.
**`.github/ISSUE_TEMPLATE/translation.yml`** — modelled on the existing `new_scraper.yml`: language name/code, offering vs requesting, a checkbox acknowledging the fallback behaviour.

---

## Risks

| Risk | Disposition |
|---|---|
| `cmd/scrape.go:749-782` (`proceedAfterCollapse`) parses hardcoded `y`/`yes` | **Not touched.** The prompt is runtime text and stays English, so no bug goes live. Recorded as the first prerequisite for any future runtime-string pass, with a `// See docs/translations.md` comment beside the switch. |
| Translated `{{` panics the help renderer | Three layers: `safe()` strips at runtime, `TestCatalogsHaveNoTemplateDelimiters` rejects at PR time, `TestEveryCatalogRendersHelp` renders every command in every language. |
| Cobra upgrade changes `"help for %s"` or the usage template | `TestCobraDefaultsUnchanged` + `TestUsageTemplateMatchesCobra` → red test, not silently-English help. |
| Shared `rootCmd` mutated by one test poisons the next | `Localize` returns a restore func; the hygiene pattern (`t.Cleanup(restore)` + reset language) is mandatory in `cmd/i18n_localize_test.go`; `TestLocalizeRestoreRoundTrip` proves restore is total. Note `Localize` is **not idempotent across languages without restore** (keys are English source strings), which is fine: production calls it once per process. |
| Ambient `LANG=fr_FR` user nagged by "unknown language" warning | Warning is gated on `explicit` — only `--lang`, `FSS_LANG`, and config `language:` can trigger it. |
| Completion descriptions come from `Short` | **Desirable and safe** — served at completion time by the live `fss __complete` process. Noted in the doc. |
| `misspell` on Korean text | **No `.golangci.yml` change needed** — golangci-lint analyses Go source; catalogs are `.json`. Verify once with `make lint` after adding `ko.json`. |
| Tests shelling out to `--help` | **None exist**; and `Localize` is only reachable from `cmd.Execute()`, which no test calls. |
| CJK alignment / wrapping | **Verified non-issue** (see Context: pflag never wraps, padding is ASCII-derived). |
| `docs/usage.md` (869 lines) drifts | Non-goal. English remains canonical. |

## Implementer pitfalls (read before coding)

1. `//go:embed locales/*.json` fails the build if no file matches — commit `locales/_template.json` containing `{}` in step 1, before the extractor exists.
2. The `raw` candidate regex in `SetLanguage` (`^[A-Za-z0-9_]{1,16}$`) is what lets tests load `_pseudo`; don't "simplify" it away, and don't let any string outside that regex reach `localeFS.ReadFile`.
3. Never read the `--lang` flag with `GetString` — resolution must complete before cobra parses anything.
4. `Keys` must emit the parameterized consts (`"help for %s"`), never the concatenated instances (`"help for fss"`); `Localize` must do the reverse (translate the format, `Sprintf` the name back in).
5. In tests, `rootCmd.SetOut(nil)` / `SetErr(nil)` restores stdout/stderr — do it in `t.Cleanup`, or later tests print into a dead buffer.
6. `json.MarshalIndent` output has no trailing newline — append one, or the sync test loops forever against editor-saved files.
7. Don't reorder or reformat cobra's template structure "for readability" — `TestUsageTemplateMatchesCobra` is byte-exact, deliberately.
8. `t.Setenv` + `xdg.Reload()`: use the existing `isolateXDG` helper for `LanguagePref` tests; plain `t.Setenv` alone does nothing because `adrg/xdg` caches paths at init.
9. Keep every English source string byte-exact when it becomes a key (including the `\n` in multi-line `Long`s) — the catalog lookup is exact-match.

---

## Task order

Each step ends green: `make test && make lint`.

1. **`internal/i18n`**: `i18n.go` per spec + placeholder `locales/_template.json` (`{}`) + `i18n_test.go` — `Normalize` table incl. `ko_KR.UTF-8`, `ko-KR`, `KO_kr`, `C`, `POSIX`, `""`, `../etc/passwd`, 17-byte input; `SetLanguage` for `""`, `en`, unknown, and (once `ko.json` exists, step 8) exact/base fallback; `T` fallback on missing **and** empty; `Available` ordering.
2. **`internal/i18n/cobrai18n`**: `cobrai18n.go` — `visit`, `prepare`, `Keys`, `Localize` (+restore), `usageTemplate`, `safe`, chrome consts, `translateUseLine = false`; `cobrai18n_test.go` (synthetic-tree tests listed above).
3. **`internal/config`**: `Language` field, hoist `maxConfigBytes`, `LanguagePref()`; `TestLanguageKeyIsKnown` + `TestLanguagePref`.
4. **`cmd/i18n.go`** + `cmd/i18n_test.go` (`langFromArgs`, `resolveLanguage`).
5. **`cmd/root.go`**: register `--lang` in `init()`, rewire `Execute()` per spec. *(Only this file is modified under `cmd/`; steps 4/6/7 only add files.)*
6. **`cmd/i18n_template_test.go`**: run it once; commit the generated `_template.json` and `_pseudo.json`.
7. **`internal/i18n/catalog_test.go`** + **`cmd/i18n_localize_test.go`** (validation + real-tree render tests; `_pseudo` exists now, so all pass).
8. **`locales/ko.json`**: translate from the template (machine-assisted is fine; keep placeholders, no backticks, no `{{`); re-run all tests.
9. **`Makefile`**: `i18n-extract` target **and** the awk class fix at `Makefile:34`; comment blocks in `config.example.yaml` + `cmd/config_default.yaml`.
10. **Docs**: `docs/translations.md`, `CONTRIBUTING.md` section, `README.md` bullet, `.github/ISSUE_TEMPLATE/translation.yml`.

Dependencies: 1–2 and 3 are independent; 4–5 need 1+3; 6 needs 2+5; 7 needs 6; 8 needs 6.

## Verification

```sh
make test && make lint
```

Then manually (`go build -o fss .` first):

```sh
./fss --help                              # baseline
./fss scrape --help ; ./fss stash import --help ; ./fss help scrape
./fss --lang ko --help                    # Korean chrome + descriptions
FSS_LANG=ko ./fss stash --help
LANG=ko_KR.UTF-8 ./fss --help             # base-language fallback via LANG
LANG=fr_FR.UTF-8 ./fss --help             # English, NO warning (ambient source)
./fss --lang xx --help                    # warns on stderr, renders English
./fss --lang ko badcmd                    # Korean "Error:" prefix, English cobra message
LANG=ko_KR.UTF-8 ./fss completion zsh | head   # script generation unaffected
```

**The acceptance gate**: with no language set, `--help` output must be **byte-identical** to HEAD-before-this-work. Compare against a clean baseline build (a plain `git stash` won't stash the new untracked files):

```sh
git worktree add /tmp/fss-baseline <commit-before-i18n>
(cd /tmp/fss-baseline && go build -o /tmp/fss-old .)
go build -o /tmp/fss-new .
for args in "--help" "scrape --help" "stash import --help" "badcmd"; do
  diff <(env -u LANG -u LC_ALL -u LC_MESSAGES -u FSS_LANG /tmp/fss-old $args 2>&1) \
       <(env -u LANG -u LC_ALL -u LC_MESSAGES -u FSS_LANG /tmp/fss-new $args 2>&1) \
    && echo "OK: $args"
done
git worktree remove /tmp/fss-baseline
```
