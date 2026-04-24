# Security Policy

## Credentials

FSS handles credentials in two places:

- **Stash API key** — authenticates against your Stash instance
- **Site API keys** — some scrapers extract rotating API keys from site HTML at runtime (e.g., Algolia keys for Gamma Entertainment sites)

### Best practices

- Use the `FSS_STASH_API_KEY` environment variable instead of putting your API key in `config.yaml`. Config files on disk are readable by any process running as your user.
- Never commit `config.yaml` to version control. The `.gitignore` does not cover it since it lives outside the repo (XDG config directory), but be careful if you copy it into the project.
- Site API keys extracted at runtime are ephemeral and not stored.

## Network

FSS makes outbound HTTP requests to:

- Scraper target sites (the studio URLs you provide)
- Algolia's API (`TSMKFA364Q-dsn.algolia.net`) for Gamma Entertainment scrapers
- Your Stash instance (default `localhost:9999`)

No data is sent to any analytics services, or any other third party.

## Reporting a vulnerability

If you find a security issue, please open a GitHub issue or email the maintainer directly. There is no bug bounty program.
