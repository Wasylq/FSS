# Security Policy

## Credentials

FSS handles credentials in two places:

- **Stash API key** — authenticates against your Stash instance
- **Site API keys** — some scrapers extract rotating API keys from site HTML at runtime (e.g., Algolia keys for Gamma Entertainment sites)

### Best practices

- Use the `FSS_STASH_API_KEY` environment variable instead of putting your API key in `config.yaml`. Config files on disk are readable by any process running as your user.
- Never commit `config.yaml` to version control. The `.gitignore` does not cover it since it lives outside the repo (XDG config directory), but be careful if you copy it into the project.
- Site API keys extracted at runtime are ephemeral and not stored.

## User-Agent

FSS defaults to a Firefox user-agent string. You can override it in `config.yaml`:

```yaml
user_agent: "chrome"              # use the built-in Chrome UA
user_agent: "MyBot/1.0"           # use a custom string
user_agent: ""                    # default: Firefox UA
```

The UA string is sent with every HTTP request to scraper target sites. It does not affect requests to your Stash instance.

## Network

FSS makes outbound HTTP requests to:

- Scraper target sites (the studio URLs you provide)
- Algolia's API (`TSMKFA364Q-dsn.algolia.net`) for Gamma Entertainment scrapers
- Your Stash instance (default `localhost:9999`)

No data is sent to any analytics services, or any other third party.

### SSRF mitigations

`fss stash import --cover` downloads thumbnail URLs from FSS JSON files and pushes them to Stash. When importing third-party JSON dumps, a malicious URL could target internal services. FSS validates cover URLs before fetching:

- **Scheme**: only `http` and `https` are allowed. `file://`, `gopher://`, `data:`, etc. are rejected.
- **Host**: must not resolve to a private (RFC1918), loopback, link-local, or unspecified IP address. This blocks `localhost`, cloud metadata endpoints (`169.254.169.254`), and internal network hosts.
- **Size**: response bodies are capped at 10 MiB.

If your cover images are hosted on a LAN media server (e.g. `192.168.1.50`), pass `--cover-allow-private` to bypass the IP check. The scheme and size restrictions still apply.

See [docs/stash.md — Cover images](docs/stash.md#cover-images) for full details. The implementation is in `stash/client.go:validateCoverURL`.

**Limitation**: the DNS lookup happens before the HTTP request. DNS rebinding attacks (where a hostname resolves to a public IP during validation but to a private IP during the actual fetch) are not mitigated. For the expected threat model — importing someone else's JSON dump — the attacker would also need to control DNS for a domain the user resolves, which is a low-probability scenario.

## Reporting a vulnerability

If you find a security issue, please open a GitHub issue or email the maintainer directly. There is no bug bounty program.
