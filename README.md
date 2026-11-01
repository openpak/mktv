# OpenKart

OpenPak's own website for Mario Kart 8 (Wii U) highlights, ghosts, rankings and tournaments, live at <https://openkart.openpak.org>.

| | |
| --- | --- |
| Network | Wii U |
| Role | Public website (repo `mktv`, site name OpenKart): Mario Kart TV highlights, time-trial ghosts and rankings per course, tournaments |
| Depends on | the [`mario-kart-8`](https://github.com/openpak/nex/tree/main/titles/mario-kart-8) title's read API; OpenPak's public API for sign-in |
| Image | `ghcr.io/openpak/mktv` |
| Based on | nothing: written from scratch (see [Clean room](docs/clean-room.md)) |
| Licence | AGPL-3.0-only |

## What it shows

| Page | What |
| --- | --- |
| `/` | latest highlights, courses with times, latest tournaments |
| `/highlights`, `/highlights/{id}` | highlight reels: player, date, size, tags, thumbnail if the game uploaded one, an embedded YouTube video when one is stored |
| `/courses`, `/courses/{n}` | ranking per course (fastest first), the ghosts uploaded for it, your own standing when signed in |
| `/tournaments`, `/tournaments/{id}` | tournaments and one season's leaderboard |
| `/signin`, `/me` | sign in with an OpenPak account; your times and highlights |

Replays are not playable in a browser. See [Gaps](docs/gaps.md).

## Run

```sh
cp example.env .env   # point MKTV_API_URL at the title's read API
set -a; . ./.env; set +a
GOWORK=off go run ./cmd/mktv
```

The stylesheet is openpak.org's own, copied by `scripts/sync-css.sh` so the two stay one identity.

## Docs

- [Docs index](docs/README.md)
- [Read API](docs/api.md)
- [Deploy](docs/deploy.md)
- [Gaps](docs/gaps.md)
- [Clean room](docs/clean-room.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
