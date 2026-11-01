# Read APIs

OpenKart reads one JSON API per game. All are read-only, on the container network, never published; a game with no address configured is not offered. Every list is `{total, offset, limit, items}`; `limit` defaults to 20 and is capped at 100 (the course and player-ranking lists may also be a bare array or `{items}`; the site accepts both). PIDs are strings. Errors are `{"error": "..."}` with 400 (bad parameter), 404, 503 (store down; no detail leaked), 501 (no object store configured: the site shows an empty list).

| Game | Env on mktv | Server | Paths | Auth |
| --- | --- | --- | --- | --- |
| Mario Kart 8 (Wii U) | `MKTV_API_URL` | `mario-kart-8` (nex), `MK8_API_ADDR`, production `:22853` | `/api/v1/...` | none |
| Mario Kart 8 Deluxe (Switch) | `MKTV_MK8D_API_URL` | switch-nex, `MK8D_API_ADDR`, `:22854` | `/api/v1/...` | none |
| Mario Kart 7 (3DS) | `MKTV_MK7_API_URL` | `mario-kart-7` (nex), `MK7_API_ADDR`, `:22855` | `/api/v1/...` | none |
| Mario Kart Wii | `MKTV_MKW_API_URL`, `MKTV_MKW_API_KEY` | wfc-race, guarded HTTP `:20208` | `/api/mkw/v1/...` | header `X-WFC-Internal-Key` |

## What the site needs from each API

Paths below are relative to the game's prefix (`/api/v1` or `/api/mkw/v1`).

| Endpoint | Used for | Needed by |
| --- | --- | --- |
| `GET /courses` | `[{course, ranked, ghosts}]`: course lists, game home counts, the ghost list | all |
| `GET /courses/{n}/rankings?limit&offset` | `{rank, player, time_ms, time, updated, name?}`, fastest first. `name` is shown instead of "Player <id>" when set (Mario Kart Wii: the Mii name) | all |
| `GET /courses/{n}/ghosts?owner&limit&offset` | ghost objects for a course (list and course page) | all |
| `GET /objects/{id}/body` | the ghost's bytes (thumbnails for highlights); the site always serves non-images as an attachment | all |
| `GET /players/{pid}/rankings` | `{course, rank, of, time}` per course, for "Your results" and the standing on a course page. `pid` is the Wii U PID (Mario Kart 8), the 3DS PID (7; the Wii U PID unless the account has its own), the decimal Switch id (8 Deluxe) or the Wii console user id (Wii) | all |
| `GET /tournaments?limit&offset`, `GET /tournaments/{id}?season=` | tournaments (Mario Kart 7: communities) and one season's top 100 | Mario Kart 8, 8 Deluxe, 7 |
| `GET /mktv?owner=`, `GET /objects/{id}` | Mario Kart TV highlights and one object | Mario Kart 8 only |

An object is `{id, owner, name, size, data_type, slot, tags, meta_size, meta_hex, thumbnail, rating, youtube_id, created, updated}` plus optional ghost fields `{course, time_ms, time, name}`; the site uses `id`, `owner`, `name`, `size`, `time`, `updated` for ghosts.

## Mario Kart 8: what is a fact and what is inferred

Facts (NintendoClients by Kinnay, MIT: `ranking_mk8d.py`, `matchmaking_mk8d.py`): rankings are the Ranking protocol with the category as the course and ascending scores; time-trial scores are milliseconds; tournaments are `SimpleSearchObject` rows with competition leaderboards (higher score first).

Inferred, because no retail console has uploaded to this server yet (the production DataStore holds no objects): a ghost is a DataStore object with a persistence slot (key scheme `ghosts/<pid>/<slot>.bin`) whose `data_type` is the course; a highlight is a public finished object with no persistence slot that refers to no other object (`MK8_MKTV_DATA_TYPE` narrows it to one `data_type` once it is known); a thumbnail is an object whose `refer_data_id` is the highlight. Every object carries its raw `data_type`, `tags` and metadata size so the site can be corrected without a server change.

`youtube_id` on an object is reserved for the highlight upload feature and is empty until it exists; the site embeds it only when it is an 11-character id.

## Other games

Course ids per game, FACTS versus inferred:

- **Mario Kart Wii**: `course` is wfc-common's leaderboard course id, 0 to 31 (`platforms/wii-ds/infra/wfc-common/common/mario_kart_wii.go`, 0x00 Mario Circuit to 0x1F GBA Shy Guy Beach). Fact; named on the site. `player` is the console user id that `wfc_user_id` carries.
- **Mario Kart 8 Deluxe**: `course` is the Ranking category; its mapping to a course is not confirmed. The site says "Course N". `player` is the Switch principal id as a decimal string (the 16-hex network service account id read as a number), which is `switch_pid`.
- **Mario Kart 7**: `course` is the number the game uses as the ranking category (NintendoClients: Ranking and DataStore protocols); the mapping to a course is not confirmed. Communities (NintendoClients: Mario Kart 7 communities) fill the tournaments endpoints.
