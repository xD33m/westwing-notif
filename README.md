# westwing-notif

[![CI](https://github.com/xD33m/westwing-notif/actions/workflows/ci.yml/badge.svg)](https://github.com/xD33m/westwing-notif/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A tiny, self-hosted price watcher for [westwing.de](https://www.westwing.de) product pages.
It sends a push notification through [ntfy](https://ntfy.sh) as soon as a price drops.

It is built to sit on a Raspberry Pi next to many other containers. The image is a single
static Go binary on `scratch` (~7 MB), idles at ~5 MB RAM, and uses no CPU between checks.

> **Not affiliated with Westwing.** This is an independent hobby project. See
> [Responsible use and legal notes](#responsible-use-and-legal-notes).

## Features

| Notification | When | Priority |
|---|---|---|
| Beobachte jetzt | First successful check of a newly added product | low |
| Preis gesunken | Any drop vs. the last seen price: old → new, € and %, product image attached, "Neuer Tiefstpreis!" on a new low | high |
| ausverkauft / wieder verfügbar | The stock status changes | default |
| Prüfung schlägt fehl / wieder erreichbar | 3 failed checks in a row, and when checks recover | default |
| Westwing-Wochenübersicht | Weekly (default Sunday 10:00): every product's price, lowest price and stock. A missing summary means the monitor is down | low |

Notification texts are in German, matching the shop.

- Watches any number of products. The list is re-read before every check, so no restart is needed.
- Checks every 2 hours by default, with ±15 % random jitter and a pause between products.
- Reads the price from the page's [schema.org](https://schema.org/Product) structured data, not from the visual layout.
- Keeps state in plain files: `state.json` for last/lowest price and `history.csv` with one row per check.
- If ntfy can't be reached, nothing is lost: the drop is detected again on the next check.
- Price increases are recorded but not pushed. Discount codes are not tracked.

## Quick start (Docker Compose)

You need Docker with Compose and Go 1.26+ on the machine that builds the binary.

1. Build the binary for your target. Use `GOARCH=arm64` for a 64-bit Raspberry Pi OS and `amd64` for a regular server:

   ```bash
   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o westwing-notif .
   ```

2. Pick an unguessable ntfy topic and save it in `.env`. Topics on ntfy.sh are public to anyone who knows the name:

   ```bash
   echo "NTFY_URL=https://ntfy.sh/westwing-$(od -An -tx1 -N8 /dev/urandom | tr -d ' \n')" > .env
   ```

3. Put the product URLs you want to watch in `config/items.txt`, one per line.
4. Make sure `user:` in `docker-compose.yml` matches your user (`id -u`:`id -g`), then start it:

   ```bash
   docker compose up -d --build
   ```

5. Subscribe to the topic from `.env` in the ntfy app. A "Beobachte jetzt" push confirms that it works.

### Deploying from another machine

`deploy.sh` does all of the above over SSH. It cross-compiles locally, copies the binary and
compose file to the host, creates a random topic on first run and restarts the container.
It never overwrites `.env` or `config/items.txt` on the host.

```bash
cp deploy.env.example deploy.env
```

```bash
bash deploy.sh
```

## Configuration

| Setting | Where | Default |
|---|---|---|
| `NTFY_URL` | `.env` | required, e.g. `https://ntfy.sh/<topic>` or your own ntfy server |
| `CHECK_INTERVAL` | `docker-compose.yml` | `2h` (minimum `1m`; please keep it modest) |
| `SUMMARY_SCHEDULE` | `docker-compose.yml` | `Sun 10:00`; `off` disables the weekly summary |
| `TZ` | `docker-compose.yml` | `Europe/Berlin` |
| Products | `config/items.txt` | one URL per line, `#` starts a comment line |

The container is read-only, unprivileged and capped at 32 MB RAM and a quarter CPU. Its healthcheck
fails if no product was checked successfully within three intervals.

## Development

```bash
go test ./...
```

Run a single check locally against a test topic:

```bash
NTFY_URL=https://ntfy.sh/<test-topic> ITEMS_FILE=config/items.txt DATA_DIR=build go run . run -once
```

The design rationale is in [docs/design.md](docs/design.md).

## Responsible use and legal notes

- **No affiliation.** This project is not affiliated with, endorsed by or sponsored by Westwing
  or ntfy. "Westwing" is a trademark of its respective owner and is used here only to describe
  which website the tool works with.
- **Personal use.** The tool is meant for watching a handful of products for your own purchase
  decisions. It requests each product page about every two hours, like a person revisiting it.
  Don't use it for bulk scraping, price aggregation or republishing Westwing's content.
- **Terms and robots.txt.** You are responsible for using it in line with Westwing's terms of
  use. At the time of writing, westwing.de's `robots.txt` doesn't disallow individual product
  pages. If Westwing objects to automated access, stop using the tool.
- **Data.** No personal data is collected. The only data stored is product name, price, stock
  status and timestamps, kept locally in `data/`. Notifications go to the ntfy server you
  configure. Choose a private topic or a self-hosted server.
- **Test fixture.** `testdata/libby.html` contains only the structured product data (name,
  price, availability and an image URL) needed to test the parser, not a copy of the web page.
- **No warranty.** Prices can change between a notification and checkout. Always check the shop
  before you buy. See [LICENSE](LICENSE).

## Third-party software

The binary embeds [`golang.org/x/crypto/x509roots/fallback`](https://pkg.go.dev/golang.org/x/crypto/x509roots/fallback)
(BSD-3-Clause, © The Go Authors) for TLS root certificates, and the Go standard library
(BSD-3-Clause). Their license texts apply to any binary you distribute.

## License

[MIT](LICENSE) © 2026 Lucas K
