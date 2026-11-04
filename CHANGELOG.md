# Changelog — OpenKart (mktv)

## v0.3.3 — 2026-11-04

- Phones: the game sub-navigation (Overview, Courses, Ghosts, Tournaments) becomes one native dropdown when it does not fit on a line. Pills stay on desktop and without JavaScript. New `static/pillselect.js`.

## v0.3.2 — 2026-11-03

- Deploy docs: production now serves all four games. Mario Kart 8 Deluxe runs on the host network, so mktv reaches it as `http://host.containers.internal:22854`; Mario Kart 7 needs `MK7_API_ADDR=:22855` set on its server; `MKTV_MKW_API_KEY` is wfc-race's `WFC_INTERNAL_KEY`.

## v0.3.1 — 2026-11-02

- Course rankings come from the openpak.org public rankings API (`/api/v1/rankings/{game}/{course}`), so
  OpenKart shows the same numbers and player names as openpak.org and the app. The game's own read API
  answers only if openpak.org does not. "You" highlighting also matches the OpenPak account id.

## v0.3.0 — 2026-11-01

- Four games: Mario Kart 8 (Wii U), Mario Kart 8 Deluxe (Switch), Mario Kart 7 (3DS) and Mario Kart Wii. A game switcher in the top bar (Wii U, Switch, 3DS, Wii), a section bar per game, game cards on the home page and a home page per game with counts. Mario Kart 8 keeps `/highlights`, `/courses`, `/tournaments`, `/me`; the others live under `/mk8d/`, `/mk7/`, `/mkw/`
- A game with no API address is not offered; a game whose server does not answer shows a friendly "not answering right now" page, and the other games stay up
- New Ghosts section (`/ghosts`, `/{game}/ghosts`) with a download button; ghost files are always served as an attachment
- Tournaments for Mario Kart 8, 8 Deluxe and 7 (communities there); none for Wii
- "Your results" covers every game: Wii U PID (Mario Kart 8), 3DS PID (Mario Kart 7; `n3ds_pid` if present, else the Wii U PID, which is the same PNID), Switch id (8 Deluxe), Wii console user id (Mario Kart Wii). A game with no linked console says so, with a link to openpak.org
- The session cookie carries `wiiu_pid`, `n3ds_pid`, `switch_pid`, `wfc_user_id` (all optional, strings or numbers) and stays under 400 bytes
- Course names: Mario Kart Wii is named (wfc-common order); the other games stay "Course N" until the ranking category mapping is confirmed on a console (docs/gaps.md)
- Mario Kart Wii: `/api/mkw/v1/...` with the internal key header
- New env: `MKTV_MK8D_API_URL`, `MKTV_MK7_API_URL`, `MKTV_MKW_API_URL`, `MKTV_MKW_API_KEY`, `MKTV_<GAME>_COURSES_FILE`. `MKTV_API_URL` (Mario Kart 8) is now optional; at least one game is needed
- Description, about page and footer text name the four games
- Friendly 404 page for unknown addresses
- Body text is sans-serif again: base.css now defines `--font-sans` as openpak.org does; `scripts/sync-css.sh` copies it from the source, and a test checks it

## v0.2.3 — 2026-11-01

- Sign out ends only this site's session and returns to the home page; it no longer goes through openpak.org's "Sign out of OpenPak?" page, and openpak.org stays signed in. The next Sign in may sign straight back in while openpak.org is signed in

## v0.2.2 — 2026-11-01

- Footer line "Part of the OpenPak network" moved to the bottom line, as on the other Open sites

## v0.2.1 — 2026-11-01

- The site's only address is kart.openpak.org (was openkart.openpak.org): default public URL, sign-in redirect and docs
- Footer: "Part of the OpenPak network", linking to openpak.org

## v0.2.0 — 2026-11-01

- Sign-in through openpak.org single sign-on (`/auth/login`, `/auth/callback`, `/auth/logout`; state and PKCE, server-side code exchange). The top bar has the same Sign in button and account chip as openpak.org; the site has no sign-in form
- "Your results" (`/me`) is back, keyed by the Wii U player ID the account carries; an account with no Wii U linked is told so, with a link to openpak.org
- New: `SSO_CLIENT_OPENKART_SECRET`. `MKTV_CORE_URL` is now openpak.org's website, server to server

## v0.1.3 — 2026-11-01

- Removed the site's own sign-in form, the "Your results" page and sign-out. Sign-in comes back through openpak.org's single sign-on, not a separate form

## v0.1.2 — 2026-11-01

- Top bar and footer show only the OpenKart mark and name; OpenKart favicon and touch icon
- Footer follows the shared Open-site layout (OpenPak, OpenKart, Project, Legal)

## v0.1.1 — 2026-11-01

- Default port 20190 (20150 belongs to connectivity)

## v0.1.0 — 2026-11-01

- First release: highlights, courses (ranking and ghosts), tournaments, About
- Sign-in through OpenPak core accounts (password and two-factor); `/me` shows your times and highlights
- openpak.org's header, footer and stylesheet (`scripts/sync-css.sh`), light and dark, M PLUS Rounded 1c self-hosted
- YouTube embed slot per highlight, drawn only when the API reports a video id
- Tag-only `release.yml` publishing `ghcr.io/openpak/mktv`
