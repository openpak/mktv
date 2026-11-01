# OpenKart

OpenPak's own website for Mario Kart 8 (Wii U), Mario Kart 8 Deluxe (Switch), Mario Kart 7 (3DS) and Mario Kart Wii: rankings, ghosts, tournaments and communities, live at <https://kart.openpak.org>.

| | |
| --- | --- |
| Network | Wii U, Switch, 3DS, Wii |
| Role | Public website (repo `mktv`, site name OpenKart): time-trial rankings per course, downloadable ghosts, tournaments or communities, and (Mario Kart 8 only) Mario Kart TV highlights |
| Depends on | one read API per game (Mario Kart 8 and 7: the `nex` titles; 8 Deluxe: `switch-nex`; Wii: `wfc-race`); openpak.org single sign-on |
| Image | `ghcr.io/openpak/mktv` |
| Based on | nothing: written from scratch (see [Clean room](docs/clean-room.md)) |
| Licence | AGPL-3.0-only |

## What it shows

A game is offered (top bar, home cards) only when its API address is set. Mario Kart 8 keeps its original URLs; the other games live under `/{game}/`.

| Page (Mario Kart 8 / others) | What |
| --- | --- |
| `/` | the games, as cards (needs no game server) |
| `/mk8`, `/mk8d`, `/mk7`, `/mkw` | one game: counts, courses, latest highlights and tournaments |
| `/courses`, `/courses/{n}` / `/{game}/courses[/{n}]` | ranking per course (fastest first), the ghosts for it, your standing when signed in |
| `/ghosts` / `/{game}/ghosts` | uploaded ghosts per course, with a download button (served as a file) |
| `/tournaments[/{id}]` / `/{game}/tournaments[/{id}]` | tournaments (Mario Kart 7: communities) and one season's leaderboard; not for Wii |
| `/highlights[/{id}]` | Mario Kart TV highlight reels, Mario Kart 8 only |
| `/auth/login`, `/auth/callback`, `/auth/logout`, `/me` | sign in through openpak.org; your results in every offered game |

Replays are not playable in a browser. See [Gaps](docs/gaps.md).

## Run

```sh
cp example.env .env   # point the MKTV_*_API_URL variables at the games' read APIs
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
