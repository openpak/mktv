# Changelog — OpenKart (mktv)

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
