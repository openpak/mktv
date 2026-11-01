package site

import "strconv"

// Game is one Mario Kart title the site can show. A game is offered only when its read API
// address is configured.
type Game struct {
	Slug                            string // mk8, mk8d, mk7, mkw
	Name                            string // Mario Kart 8 Deluxe
	Console                         string // Switch (the switcher label)
	Home                            string // /mk8d
	Base                            string // "" for Mario Kart 8 (its pages keep the original URLs), /mk8d for the others
	Intro                           string // one sentence for the home card
	Highlights, Ghosts, Tournaments bool
	TournamentsLabel                string // Tournaments, or Communities
	Count                           int    // courses in the game
	Names                           []string

	api   *apiClient
	names map[int]string // from the optional courses file, wins over Names
	// ident picks the visitor's player id for this game out of the signed-in player.
	ident func(Player) string
}

// gameDef is the fixed description of a game; New fills in the API client.
type gameDef struct {
	Game
	prefix string
}

// The course tables are FACTS only where marked. Counts: Mario Kart 8 (Wii U) has 48 courses (32 on
// the disc, 16 from the two DLC packs), Mario Kart 8 Deluxe has 96 (48 on the disc, 48 from the
// Booster Course Pass), Mario Kart 7 has 32 (8 cups of 4) and Mario Kart Wii has 32 (8 cups of 4).
//
// Names: only Mario Kart Wii has a documented id to course mapping in this workspace: the
// leaderboard course ids of wfc-common (platforms/wii-ds/infra/wfc-common/common/mario_kart_wii.go,
// 0x00 Mario Circuit to 0x1F GBA Shy Guy Beach), which the Wii's own ranking uses. For Mario Kart 8,
// 8 Deluxe and 7 the number the game uses as the ranking category has no mapping in NintendoClients
// or in any capture we hold, so those pages say "Course N" until a console upload confirms it
// (docs/gaps.md). MKTV_COURSES_FILE and MKTV_<GAME>_COURSES_FILE can name them once it is known.
var mkwCourses = []string{
	"Mario Circuit", "Moo Moo Meadows", "Mushroom Gorge", "Grumble Volcano", "Toad's Factory", "Coconut Mall",
	"DK Summit", "Wario's Gold Mine", "Luigi Circuit", "Daisy Circuit", "Moonview Highway", "Maple Treeway",
	"Bowser's Castle", "Rainbow Road", "Dry Dry Ruins", "Koopa Cape", "GCN Peach Beach", "GCN Mario Circuit",
	"GCN Waluigi Stadium", "GCN DK Mountain", "DS Yoshi Falls", "DS Desert Hills", "DS Peach Gardens",
	"DS Delfino Square", "SNES Mario Circuit 3", "SNES Ghost Valley 2", "N64 Mario Raceway", "N64 Sherbet Land",
	"N64 Bowser's Castle", "N64 DK's Jungle Parkway", "GBA Bowser Castle 3", "GBA Shy Guy Beach",
}

func gameDefs() []gameDef {
	return []gameDef{
		{Game: Game{Slug: "mk8", Name: "Mario Kart 8", Console: "Wii U", Home: "/mk8", Base: "",
			Intro:      "Mario Kart TV highlights, time-trial rankings, ghosts and community tournaments.",
			Highlights: true, Ghosts: true, Tournaments: true, TournamentsLabel: "Tournaments", Count: 48,
			ident: func(p Player) string { return p.PID }}, prefix: "/api/v1"},
		{Game: Game{Slug: "mk8d", Name: "Mario Kart 8 Deluxe", Console: "Switch", Home: "/mk8d", Base: "/mk8d",
			Intro:  "Time-trial rankings, ghosts and tournaments.",
			Ghosts: true, Tournaments: true, TournamentsLabel: "Tournaments", Count: 96,
			ident: func(p Player) string { return p.SwitchPID }}, prefix: "/api/v1"},
		{Game: Game{Slug: "mk7", Name: "Mario Kart 7", Console: "3DS", Home: "/mk7", Base: "/mk7",
			Intro:  "Time-trial rankings, ghosts and communities.",
			Ghosts: true, Tournaments: true, TournamentsLabel: "Communities", Count: 32,
			ident: func(p Player) string { return p.N3DSPID() }}, prefix: "/api/v1"},
		{Game: Game{Slug: "mkw", Name: "Mario Kart Wii", Console: "Wii", Home: "/mkw", Base: "/mkw",
			Intro:  "Worldwide time-trial rankings and ghosts for all 32 courses.",
			Ghosts: true, Count: 32, Names: mkwCourses,
			ident: func(p Player) string { return p.WFCUserID }}, prefix: "/api/mkw/v1"},
	}
}

// CourseName is the course's name, or "Course N" where the id to course mapping is not documented.
func (g *Game) CourseName(n int) string {
	if name, ok := g.names[n]; ok {
		return name
	}
	if n >= 0 && n < len(g.Names) {
		return g.Names[n]
	}
	return "Course " + strconv.Itoa(n)
}

// Identity is the visitor's player id for this game, "" when the account has none linked.
func (g *Game) Identity(p *Player) string {
	if p == nil {
		return ""
	}
	return g.ident(*p)
}
