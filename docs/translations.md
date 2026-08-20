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
2. Translate the values, leaving the keys untouched. **Translate only what
   you're confident about** and leave the rest — a value still equal to its
   English key renders as English, which is exactly right.
3. Run `make test`.
4. Open a PR.

You need no Go knowledge: the keys *are* the English strings, and the file is
flat JSON.

The template ships with each value set to its own key, so the file reads as
English out of the box and every entry shows the string you are translating.
If you would rather track what is left to do, setting a value to `""` also
falls back to English — an empty value and a missing key behave identically —
so you can blank the ones you have not reached and fill them in as you go.

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

## Translation platform (Weblate)

Not enabled yet — the PR path above is the whole workflow today. This section
is the runbook for turning it on, and `.weblate` in the repo root is its
companion, a `wlc` client config naming a component that does not exist yet.

**Why Weblate**: it is GPL like fss, self-hostable, and hosted.weblate.org
offers free hosting to libre projects. It contributes back as GitHub pull
requests, so CI gates every translation exactly as it gates a hand-written one.

**Why it is not on yet**: at ~113 strings and one language, the setup costs more
than it saves. The point at which it pays off is the third language, or the
first translator who will not touch git.

Weblate's UI changes between releases, so treat the step names below as
landmarks and the **values** in the settings table as the part that matters.
Check <https://docs.weblate.org/> for current specifics.

### Setup

1. **Get a Weblate.** Either request free libre hosting at
   <https://hosted.weblate.org/> (a public repo under an OSI licence — fss is
   GPL-3.0 — reviewed by a human, so allow a few days), or self-host: Weblate
   ships an official Docker Compose stack, which is not much more work than
   what is already in [docker.md](docker.md).

2. **Give Weblate push access to the repo.** Weblate authenticates *to* GitHub,
   never the reverse — see Credentials below. On hosted Weblate this means
   granting their bot account access to `Anastylosis/FSS`; self-hosted, it means
   giving your instance a deploy key or a token. Nothing is added to the repo.

3. **Create a project** (`FSS`) and inside it a **component** (`Help text`)
   pointing at `https://github.com/Anastylosis/FSS.git`, branch `master`, with
   the settings in the table below.

4. **Add the webhook** so Weblate notices when master's English strings move,
   rather than waiting for its polling interval. GitHub → Settings → Webhooks →
   payload URL `https://hosted.weblate.org/hooks/github/` (or your instance's
   equivalent), content type `application/json`, the push event only. Weblate
   can add this itself if it has admin access; it does not need admin for
   anything else, so adding it by hand is the smaller grant.

5. **Open a throwaway translation and let it PR.** Before announcing anything,
   confirm the round trip: change one string in Weblate, let it open the pull
   request, and check that CI runs the catalog tests against it. That is the
   whole safety story — if it holds, a bad translation cannot reach master.

### Component settings

| Setting | Value |
|---|---|
| File format | JSON file (flat, monolingual) |
| File mask | `internal/i18n/locales/*.json` |
| Monolingual base language file | `internal/i18n/locales/_template.json` |
| Template for new translations | `internal/i18n/locales/_template.json` |
| Edit base file | off |
| Source language | English |
| Language filter | `^[a-z]{2}(_[A-Z]{2})?$` |
| Push method | GitHub pull request |
| Merge style | rebase |

Three of those are load-bearing:

- **The language filter is not optional.** The file mask treats `*` as the
  language code, so it would otherwise match `_pseudo.json` and
  `_template.json` and present them as languages called `_pseudo` and
  `_template`. `_pseudo` is a generated test fixture; a translator editing it
  breaks `TestI18nTemplateInSync` and gets a baffling CI failure.
- **`_template.json` is the base file, and Weblate must not edit it.** It is
  generated by `make i18n-extract` from the command tree, so any edit there is
  overwritten by the next regeneration. Leave *Edit base file* off. This is
  also why the template's values are the English source strings rather than
  `""` — a monolingual base file has to carry the text being translated.
- **Rebase, not merge.** Weblate's branch and master both touch files under
  `locales/`, and a regeneration on master while a translation PR is open is
  the one routine conflict this setup produces. Rebasing keeps it to the
  catalog actually being translated.

The **Cleanup translation files** addon is worth enabling: it drops keys absent
from the base file, the same invariant `TestCatalogKeysAreSubsetOfTemplate`
enforces.

### Credentials

**Nothing goes into GitHub repository secrets.** Weblate pushes to GitHub, so
Weblate holds the credential; GitHub never calls Weblate and has nothing to
authenticate with. fss's workflows reference no secrets at all, and enabling
translations does not change that.

The `wlc` API key is the other direction — you, from your machine, driving
Weblate's API. It lives in `~/.config/weblate`, never in `.weblate` and never
in the repo. If you ever do need it in CI, create a dedicated Weblate user for
it rather than using your own: an API key carries that user's access across
every project they can reach. Note also that GitHub withholds secrets from
`pull_request` runs originating in a fork — which is where translation PRs come
from — so such a workflow would silently do nothing on the very PRs it targets.
Do not reach for `pull_request_target` to work around that while checking out
the PR's code; that combination is how public repos leak secrets.

### Living with it

Weblate commits under its own account with translators recorded as co-authors,
so it changes who appears in `git log`.

When an English string is reworded, master's `_template.json` changes and
Weblate picks up the new source string on its next pull. The old key disappears
and its translations go with it — that is
[When English changes](#when-english-changes), and it is unchanged by having a
platform. Regenerate, then fix up the catalogs; Weblate will show the affected
strings as untranslated rather than silently keeping stale text.

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
