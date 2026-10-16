# Design notes

## Goal

Get a push notification the moment a watched Westwing product gets cheaper, from a
service that runs unattended on a Raspberry Pi that already hosts many other containers.

## Constraints

- **Near-zero footprint.** Target image < 15 MB and idle RAM < 10 MB. Measured: ~7 MB image
  and ~5 MB RAM.
- **No UI.** The ntfy app is the only interface. No ports, no web server, no database.
- **Never fail silently.** If the tool stops working, the user must find out.

## Decisions

| Decision | Why |
|---|---|
| Go, static binary, `FROM scratch` | Smallest practical runtime. CA roots (`x509roots/fallback`) and tz data (`time/tzdata`) are embedded because `scratch` has neither. |
| Cross-compile on the dev machine | The Pi never pulls a Go toolchain image. Its `docker build` is a single `COPY`. |
| Parse schema.org JSON-LD | Structured data is far more stable than page layout. With several offers, the lowest price is used, together with that offer's stock status, so results are deterministic. |
| Compare against the last seen price | "Any drop" is the requirement. 1049 → 1099 → 1049 pushes on the way back down. |
| Advance state only after delivery | If ntfy is unreachable, the drop is detected again on the next check instead of being lost. |
| Treat redirects as failures | A discontinued product that redirects to another product must not look like a price drop. |
| Alert after 3 consecutive failures, once | Catches layout changes or blocking without spamming. A recovery push follows. |
| Weekly summary | Covers what the tool can't report itself: if the container or the host is down, the summary doesn't arrive. |
| JSON publishing to ntfy | Titles contain `€` and umlauts, which HTTP headers can't carry. |
| Files instead of a database | `state.json` is written atomically (temp file + rename). `history.csv` is append-only. |
| 2 h interval with ±15 % jitter | Polite to the shop and frequent enough for furniture prices. |

## Out of scope

- Discount codes and member prices. Only the listed product price counts.
- Shops other than westwing.de.
- Price-increase notifications. Increases are recorded in the history only.
