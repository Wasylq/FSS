# Creators

One person often sells the same catalogue on several storefronts. Mara Vance
has a Clips4Sale studio, a ManyVids profile, a LoyalFans page and her own site;
Ines Dahl has six. FSS keys everything on the studio URL, so out of the box
those are six unrelated studios that happen to share a name — and often do not
even do that (`VeraQuill`, `Vera Quill`, `Vera Quill Films`).

A **creator** binds those storefronts together. It is what makes

```bash
fss scrape --creator "Mara Vance"      # all four stores, one command
fss compare --from-creator "Mona Reeve"    # where is each clip cheapest?
```

possible, and it is the unit a cron job should work in.

---

## Where creators live

One YAML file per creator, in a `creators.d` directory:

```
~/.config/fss/creators.d/
├── mara-vance.yaml
├── mona-reeve.yaml
└── ines-dahl.yaml
```

Resolution order: `--creators-dir` → `creators_dir:` in `config.yaml` → the
default beside your config (`$XDG_CONFIG_HOME/fss/creators.d`). A missing
directory is not an error — creators are opt-in, and every command works
without them.

**Why a directory and not a block in `config.yaml`.** These files carry no
credentials, so a set of them can live in a git repository and be shared or
pulled; and because each creator is its own file, two people adding creators
never touch the same lines. Point `creators_dir:` at a clone to use someone
else's set, or symlink individual files out of one into your own directory.

The file name is cosmetic — the `name:` field is authoritative, so renaming a
file changes nothing. Non-YAML files are skipped, so a shared repository can
carry a `README.md`, a `LICENSE` and a `.git/` without special handling.

---

## File format

```yaml
name: Mara Vance
aliases:
  - MaraVanceVIP          # optional: other spellings --creator should match
stores:
  - url: https://clipmarket.example/studio/4021/mara-vance
  - url: https://maravance.example
    delay: 2000           # optional: ms between requests, this store only
  - url: https://fanhub.example/mara-vance
    enabled: false        # optional: skip on --creator / --all-creators runs
    note: needs a session cookie
```

| Key | Required | Meaning |
|-----|----------|---------|
| `name` | yes | The creator. Must be unique across all files, after normalisation |
| `aliases` | no | Extra spellings `--creator` and `--from-creator` will match |
| `stores` | yes | At least one storefront |
| `stores[].url` | yes | Full studio URL including the scheme |
| `stores[].delay` | no | Per-request delay in ms for this store, overriding `site_delays` and `delay` |
| `stores[].enabled` | no | `false` skips the store on creator-driven runs. Default `true` |
| `stores[].note` | no | Free text, shown by `fss creators` |

Loading fails — loudly, naming the file — on a missing `name` or `stores`, a URL
without a scheme, a duplicated URL within one file, a negative delay, or two
files claiming the same creator name or alias. Unknown keys only warn, so a file
written for a newer FSS still loads.

**A note on `stores:`.** Writing `urls:` instead is the obvious mistake this
format invites, and it is a hard error rather than a creator with no
storefronts.

---

## Bootstrapping from what you already scrape

You do not have to write these by hand. `fss creators suggest` reads your
existing scene data and proposes the files:

```bash
fss creators suggest --db              # print proposals for review
fss creators suggest --db --write      # write them into creators.d
```

Studios are grouped on two signals:

1. **Name containment** — `VeraQuill`, `Vera Quill` and `Vera Quill Films` share a
   normalised substring. Fragments shorter than five characters are ignored, so
   a three-letter studio name cannot absorb every longer name containing it.
2. **A shared dominant performer** — a performer credited on at least half a
   store's scenes. This is what catches storefronts whose names have nothing in
   common, and what pulls `Velvet Hour Studio` into the Ines Dahl group.

The creator is named after the dominant performer where there is one, since a
storefront label is often a brand (`Vera Quill Films`) or a handle
(`odettelang`); every other spelling becomes an alias.

Both signals are heuristics, so `suggest` prints for review by default and
`--write` never overwrites an existing file without `--force`. It also **names**
the studios that grouped with nothing else — a storefront can be unmistakably
someone's to a human and invisible to both signals. `Duchess Nyx` on
IWantClips is Nyx Vale's, but its name shares nothing with `Nyx Vale` and it
credits no performers at all, so nothing can link it automatically. Add the URL
to the right file by hand.

`--include-single` proposes the ungrouped studios as one-store creators instead,
which is still useful for `--all-creators`.

---

## Scraping by creator

```bash
fss scrape --creator "Mara Vance"        # one creator's stores
fss scrape --creator mara --creator mona  # several (prefixes work)
fss scrape --all-creators                 # everything defined
```

`--creator` matches the name or any alias, ignoring case, spacing and
punctuation — `"Vera Quill"`, `VeraQuill` and `vera-quill` are the same lookup. A
value that uniquely prefixes one creator resolves to it; one that prefixes
several is an error naming them, never a silent pick.

Stores are scraped sequentially, in file order, and a URL reachable more than one
way is scraped once — naming a URL explicitly alongside `--all-creators` does not
scrape it twice. Every other `scrape` flag still applies, so
`fss scrape --all-creators --refresh` works as expected.

### `--stale`: cron without a crontab per creator

```bash
fss scrape --all-creators --stale 7d
```

skips any store scraped within the last seven days and prints its reasoning:

```
  skip  https://clipmarket.example/studio/4021/mara-vance  (scraped 2d ago)
  run   https://maravance.example                          (scraped 9d ago)
  run   https://fanhub.example/mara-vance                  (never scraped)

2 of 44 studio(s) due.
```

Durations accept `h`, `m`, `s` as Go does, plus `d` and `w`. Staleness is read
from the SQLite `studios` table, the only place a last-scrape time is recorded,
so `--stale` requires `--db` or a configured `db:` rather than silently scraping
everything.

That makes one crontab line enough:

```cron
0 * * * *  fss scrape --all-creators --stale 7d --db >> ~/.local/state/fss.log 2>&1
```

The run exits without touching the network when nothing is due. Note that
unattended runs skip the coverage-collapse prompt rather than answering it —
pass `--force` if you want those saves to proceed (see
[usage.md](usage.md#broken-scraper-detection)), and prefer `enabled: false` on
any store that cannot be scraped without a human.

**Do not put a first-time full traversal on a cron timer.** There is no resume:
an interrupted traversal saves what it got but no page cursor, and on a
date-sorted site the next incremental run stops early at a known ID and reports
success without ever reaching the older remainder. Let the first scrape of a
large catalogue finish in one go.

---

## Filtering by creator

`--from-creator` narrows the scene set for every command that reads stored
scenes — `fss compare`, `fss identify`, `fss stash import`, `fss creators
suggest`:

```bash
fss identify --db --from-creator "Mona Reeve" /media/clips
fss stash import --db --from-creator "Mara Vance" --apply
```

It resolves to that creator's store URLs and then behaves exactly like listing
them under `--from-studio`. Within one flag any value matches; across flags every
flag must match.

---

## Inspecting

```bash
fss creators
```

lists what is defined, with each store's last scrape date when a database is
configured:

```
13 creator(s) in /home/you/.config/fss/creators.d

Vera Quill  (aka Vera Quill Films)
  https://clipmarket.example/studio/6055/vera-quill-films  [scraped 2026-08-16]
  https://vidvault.example/profile/380818/vera-quill/store/videos  [scraped 2026-08-16]
  https://clipstore.example/store/221/VeraQuill  [scraped 2026-08-16]
```

See [compare.md](compare.md) for what to do with the grouping once it exists.
