# Gaps

1. **Replay playback.** A Mario Kart TV highlight is a recording of inputs and state that the game's own engine replays; there is no public description of its format, and re-implementing the renderer is out of scope. The site shows metadata, a thumbnail if the game uploaded one, and a video if a player published one to YouTube (embed slot present, shown only when the API reports an id).
2. **Nothing uploaded yet.** Production's DataStore has no object store configured and no object rows, so every page is in its empty state until the first console uploads. How the game classifies ghosts, highlights and thumbnails is inferred ([api.md](api.md)) and must be checked against the first real upload.
3. **Course names.** The ranking category is the game's course number; its mapping to names is not documented in the sources used, so pages say "Course N". `MKTV_COURSES_FILE` supplies names (`<id> <name>` lines) once the numbering is confirmed.
4. **Player names.** Other players appear by player ID. Names come from the signed-in account only; a public name lookup needs a core endpoint.
5. **Sign-in.** Through openpak.org single sign-on. The site keeps only the account id, the display name and the Wii U player ID in its own signed session cookie; an account with no Wii U linked sees a note on "Your results" instead of times.
6. **Public hostname.** `openkart.openpak.org` is reached through the token-managed Cloudflare tunnel, whose hostname routes live in the Cloudflare dashboard.
