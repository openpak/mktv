# Deploy

One container, `ghcr.io/openpak/mktv`, on the `openpak.network` podman network, quadlet `openpak-mktv.container`. Releasing is tagging `vX.Y.Z`; `release.yml` builds amd64 and arm64 and the host's auto-update rolls `:latest`.

```
Image=ghcr.io/openpak/mktv:latest
AutoUpdate=registry
Network=openpak.network
IP=<pinned by deploy/pin-ips.sh --apply>
PublishPort=127.0.0.1:20190:20190
EnvironmentFile=%h/openpak/deploy/mktv.env
Environment=MKTV_API_URL=http://openpak-mario-kart-8:22853 MKTV_MK8D_API_URL=http://openpak-switch-nexd:22854 MKTV_MK7_API_URL=http://openpak-mario-kart-7:22855 MKTV_MKW_API_URL=http://openpak-wfc-race:20208 MKTV_CORE_URL=http://openpak-website:20010
```

`mktv.env` (mode 0600) holds `MKTV_SESSION_KEY` (32+ random characters), `SSO_CLIENT_OPENKART_SECRET` (the same value as in the website's `.env`) and `MKTV_MKW_API_KEY` (wfc-race's `X-WFC-Internal-Key`). Sign-in is off unless `MKTV_CORE_URL`, the secret and the key are all set.

Sign-in is openpak.org single sign-on (client `openkart`; the contract is `docs/sso.md` in the website repo): `/auth/login` redirects to `https://openpak.org/account/sso/authorize`, `/auth/callback` swaps the code at `POST $MKTV_CORE_URL/internal/sso/token`, `GET /auth/logout` ends only the site session and redirects (303) to `/`; openpak.org stays signed in. The registered redirect origin is `https://kart.openpak.org` (the only address; the old long host is gone).

The game side, one read-only API per game, on the container network and never published. A game whose address is unset is not offered, so the games can be turned on one at a time:

| Game | Server | Env on the server | Env on mktv |
| --- | --- | --- | --- |
| Mario Kart 8 (Wii U) | `openpak-mario-kart-8` (nex v1.5.0 or later) | `MK8_API_ADDR=:22853` | `MKTV_API_URL` |
| Mario Kart 8 Deluxe (Switch) | switch-nexd | `MK8D_API_ADDR=:22854` | `MKTV_MK8D_API_URL` |
| Mario Kart 7 (3DS) | `openpak-mario-kart-7` | `MK7_API_ADDR=:22855` | `MKTV_MK7_API_URL` |
| Mario Kart Wii | wfc-race (guarded HTTP, `:20208`) | its existing internal key | `MKTV_MKW_API_URL`, `MKTV_MKW_API_KEY` |

Mario Kart Wii's paths are `/api/mkw/v1/...`; the other three use `/api/v1/...` (see [api.md](api.md)). The container names above are the expected ones; check them on the box.

Routing: Traefik (`deploy/traefik/dynamic.yml`) carries a `Host(kart.openpak.org)` router on the `web` entrypoint to `http://openpak-mktv:20190`. The public name is served by the Cloudflare tunnel: add a public hostname `kart.openpak.org` -> `http://localhost:20190` in the tunnel's dashboard config (the tunnel is token-managed, so it cannot be done from the box).

Check: `curl https://kart.openpak.org/healthz` answers `ok mktv`.
