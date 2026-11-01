# Deploy

One container, `ghcr.io/openpak/mktv`, on the `openpak.network` podman network, quadlet `openpak-mktv.container`. Releasing is tagging `vX.Y.Z`; `release.yml` builds amd64 and arm64 and the host's auto-update rolls `:latest`.

```
Image=ghcr.io/openpak/mktv:latest
AutoUpdate=registry
Network=openpak.network
IP=<pinned by deploy/pin-ips.sh --apply>
PublishPort=127.0.0.1:20190:20190
EnvironmentFile=%h/openpak/deploy/.env
Environment=MKTV_API_URL=http://openpak-mario-kart-8:22853 MKTV_CORE_URL=http://openpak-website:20010 MKTV_SESSION_KEY=...
```

The title side: set `MK8_API_ADDR=:22853` on `openpak-mario-kart-8` (read-only; the image must contain the API, nex v1.5.0 or later).

Routing: Traefik (`deploy/traefik/dynamic.yml`) carries a `Host(openkart.openpak.org)` router on the `web` entrypoint to `http://openpak-mktv:20190`. The public name is served by the Cloudflare tunnel: add a public hostname `openkart.openpak.org` -> `http://localhost:20190` in the tunnel's dashboard config (the tunnel is token-managed, so it cannot be done from the box).

Check: `curl https://openkart.openpak.org/healthz` answers `ok mktv`.
