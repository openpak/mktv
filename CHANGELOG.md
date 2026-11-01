# Changelog — OpenKart (mktv)

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
