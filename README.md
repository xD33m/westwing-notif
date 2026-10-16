# westwing-notif

Watches Westwing product pages and pushes to ntfy when a price drops.
Runs as a ~7 MB `scratch` container on the Raspberry Pi, using ~5 MB RAM.

## What it pushes

| Push | When | Priority |
|---|---|---|
| Beobachte jetzt | first successful check of a new item | low |
| Preis gesunken | any drop vs. the last seen price (image attached, "Neuer Tiefstpreis!" when it is) | high |
| ausverkauft / wieder verfügbar | stock status changes | default |
| Prüfung schlägt fehl / wieder erreichbar | 3 failed checks in a row, and when it recovers | default |
| Westwing-Wochenübersicht | Sundays 10:00: every item's price, lowest price, stock. No summary = something is down | low |

Price rises are logged, not pushed. Discount codes are not tracked.

## Subscribe

In the ntfy app: add subscription, server `ntfy.sh`, topic from the Pi:

```bash
ssh pi cat westwing-notif/.env
```

## Add or remove items

Edit `~/westwing-notif/config/items.txt` on the Pi,
one URL per line. Picked up at the next check, no restart.

## Settings (`docker-compose.yml`)

- `CHECK_INTERVAL` — default `2h`, each sleep ±15 % jitter
- `SUMMARY_SCHEDULE` — default `Sun 10:00`, `off` disables
- `NTFY_URL` — in `.env` on the Pi (random topic created on first deploy)

## Data

`data/state.json` (last/lowest price per item) and `data/history.csv`
(every check) on the Pi.

## Develop and deploy

```bash
go test ./...
```

```bash
bash deploy.sh
```

`deploy.sh` cross-compiles for arm64, copies binary + compose to the Pi and
restarts the container. It never overwrites `.env` or `config/items.txt`
there. One-off local run:

```bash
NTFY_URL=https://ntfy.sh/<test-topic> ITEMS_FILE=config/items.txt DATA_DIR=build go run . run -once
```
