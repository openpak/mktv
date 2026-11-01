# Gaps

1. **Replay playback.** A Mario Kart TV highlight (Mario Kart 8 only) is a recording of inputs and state that the game's own engine replays; there is no public description of its format, and re-implementing the renderer is out of scope. The site shows metadata, a thumbnail if the game uploaded one, and a video if a player published one to YouTube (embed slot present, shown only when the API reports an id).
2. **Course names.** A ranking's category is the game's own course number. Only Mario Kart Wii has a documented mapping here (wfc-common's leaderboard course ids, 0x00 Mario Circuit to 0x1F GBA Shy Guy Beach). For the other games the pages say "Course N", on purpose, until a console upload confirms the numbering:

   | Game | Courses in the game | Names on the site |
   | --- | --- | --- |
   | Mario Kart 8 (Wii U) | 48 (32 + 16 DLC) | "Course N". Not confirmed; no public source in our docs maps the ranking category to a course |
   | Mario Kart 8 Deluxe (Switch) | 96 (48 on the disc, 48 Booster Course Pass) | "Course N". Not confirmed; the Switch game may also number the booster courses differently from the Wii U game |
   | Mario Kart 7 (3DS) | 32 (8 cups of 4) | "Course N". Not confirmed |
   | Mario Kart Wii | 32 (8 cups of 4) | named, wfc-common order |

   The course counts are facts about the games; the ids used by the servers are not. `MKTV_COURSES_FILE` (Mario Kart 8) and `MKTV_MK8D_COURSES_FILE`, `MKTV_MK7_COURSES_FILE`, `MKTV_MKW_COURSES_FILE` supply names (`<id> <name>` lines) once an upload confirms them, with no code change.
3. **Nothing uploaded yet.** Until the first console uploads, pages are in their empty states. How each game classifies ghosts, highlights and thumbnails is inferred ([api.md](api.md)) and must be checked against the first real upload of that game.
4. **Player names.** Mario Kart Wii shows the name the game stored (its Mii name) when the API gives one. Everywhere else other players appear by player ID; names for those need a public name lookup in core.
5. **Sign-in.** Through openpak.org single sign-on. The site keeps only the account id, the display name and the player ids of the linked consoles (`wiiu_pid`, `n3ds_pid` when present, `switch_pid`, `wfc_user_id`) in its own signed session cookie. A game whose console is not linked says so on "Your results", with a link to the account page. The 3DS id is `n3ds_pid` if the answer has one, else `wiiu_pid`: openpak.org's docs state one PNID serves a Wii U and a 3DS.
6. **Which sections a game has** is fixed in the site, from what each game has: Highlights (Mario Kart 8 only), Ghosts (all), Tournaments (Mario Kart 8, 8 Deluxe) or Communities (Mario Kart 7). Wii has none. If a game's API answers 404 or 501 for a list, the page shows its empty state. Whether the Switch game has tournaments we can read, and what Mario Kart 7 communities look like in the API, are assumptions to check against the servers.
7. **Ghost files** play only in their own game; the site offers them as downloads and never renders them.
8. **Public hostname.** `kart.openpak.org` is reached through the token-managed Cloudflare tunnel, whose hostname routes live in the Cloudflare dashboard.
