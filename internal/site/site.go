package site

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const pageSize = 20

// Site is the http.Handler of kart.openpak.org.
type Site struct {
	cfg   Config
	games []*Game // the games that have an API address, in switcher order
	sso   *http.Client
	pages map[string]*template.Template
	mux   *http.ServeMux
	asset string
	pub   *apiClient // the openpak.org public rankings API (/api/v1/rankings)
}

// New builds the site. It does not touch the network.
func New(cfg Config) (*Site, error) {
	s := &Site{cfg: cfg, sso: &http.Client{Timeout: 10 * time.Second}, asset: strconv.FormatInt(time.Now().Unix(), 36)}
	s.pub = newAPIClient(cfg.OpenPakURL, "/api/v1", "")
	urls := map[string]string{"mk8": cfg.APIURL, "mk8d": cfg.MK8DURL, "mk7": cfg.MK7URL, "mkw": cfg.MKWURL}
	for _, d := range gameDefs() {
		if urls[d.Slug] == "" {
			continue
		}
		g := d.Game
		key := ""
		if g.Slug == "mkw" {
			key = cfg.MKWKey
		}
		g.api = newAPIClient(urls[g.Slug], d.prefix, key)
		if f := cfg.CoursesFiles[g.Slug]; f != "" {
			b, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			g.names = parseCourses(string(b))
		}
		s.games = append(s.games, &g)
	}
	if len(s.games) == 0 {
		return nil, fmt.Errorf("no game is configured")
	}
	funcs := template.FuncMap{
		"course":   func(g *Game, n int) string { return g.CourseName(n) },
		"ago":      ago,
		"date":     func(t time.Time) string { return t.UTC().Format("2006-01-02") },
		"datetime": func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") },
		"bytes":    humanBytes,
		"yt":       youtubeEmbed,
		"add":      func(a int64, b int) int64 { return a + int64(b) },
		"mine":     func(p *Player, g *Game, pid string) bool { return mine(p, g, pid) },
		"who":      func(p *Player, g *Game, pid, name string) string { return who(p, g, pid, name) },
		"active":   func(path, href string) bool { return path == href || strings.HasPrefix(path, href+"/") },
		"plural":   plural,
		"link": func(g *Game, openpak string) struct {
			Game    *Game
			OpenPak string
		} {
			return struct {
				Game    *Game
				OpenPak string
			}{g, openpak}
		},
		"pages": pager,
	}
	layout, err := templateFS.ReadFile("templates/_layout.html")
	if err != nil {
		return nil, err
	}
	s.pages = map[string]*template.Template{}
	entries, _ := fs.ReadDir(templateFS, "templates")
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "_") {
			continue
		}
		body, err := templateFS.ReadFile("templates/" + e.Name())
		if err != nil {
			return nil, err
		}
		t, err := template.New(e.Name()).Funcs(funcs).Parse(string(layout) + string(body))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		s.pages[strings.TrimSuffix(e.Name(), ".html")] = t
	}

	sub, _ := fs.Sub(staticFS, "static")
	static := http.StripPrefix("/static/", http.FileServerFS(sub))
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /{$}", s.home)
	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { s.fail(w, r, nil, errNotFound) })
	for _, g := range s.games {
		s.routes(g)
	}
	s.mux.HandleFunc("GET /about", s.about)
	s.mux.HandleFunc("GET /me", s.me)
	s.mux.HandleFunc("GET /auth/login", s.authLogin)
	s.mux.HandleFunc("GET /auth/callback", s.authCallback)
	s.mux.HandleFunc("GET /auth/logout", s.authLogout)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok mktv") })
	s.mux.HandleFunc("GET /static/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		static.ServeHTTP(w, r)
	})
	return s, nil
}

func (s *Site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; frame-src https://www.youtube-nocookie.com; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	s.mux.ServeHTTP(w, r)
}

// view is what every template receives.
type view struct {
	Title    string
	Path     string
	Asset    string
	OpenPak  string
	Contact  string
	SignIn   bool // sign-in is configured
	Player   *Player
	Game     *Game   // the game this page is about; nil on site-wide pages
	Games    []*Game // every offered game, for the switcher
	Desc     string
	Err      string
	Flash    string
	Data     any
	NotFound bool
}

func (s *Site) render(w http.ResponseWriter, r *http.Request, g *Game, status int, name, title string, data any) {
	v := view{Game: g, Games: s.games, Title: title, Path: r.URL.Path, Asset: s.asset, OpenPak: s.cfg.OpenPakURL, Contact: s.cfg.ContactEmail,
		SignIn: s.signInOn(), Data: data}
	v.Desc = s.describe(g)
	if p, ok := s.player(r); ok {
		v.Player = &p
	}
	var buf bytes.Buffer
	if err := s.pages[name].ExecuteTemplate(&buf, "page", v); err != nil {
		log.Printf("mktv: render %s: %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// fail answers an upstream error: 404 for a missing thing, a friendly 503 page when the game's
// server does not answer. Nothing of the error reaches the visitor; it goes to the log.
func (s *Site) fail(w http.ResponseWriter, r *http.Request, g *Game, err error) {
	if errors.Is(err, errNotFound) {
		s.render(w, r, g, http.StatusNotFound, "error", "Not found", "That page does not exist, or the thing it showed is gone.")
		return
	}
	log.Printf("mktv: %s: %v", r.URL.Path, err)
	name := "The game"
	if g != nil {
		name = g.Name
	}
	s.render(w, r, g, http.StatusServiceUnavailable, "error", "Temporarily unavailable", name+" is not answering right now. Try again in a minute.")
}

// describe is the meta description: nominative, naming the games this site shows.
func (s *Site) describe(g *Game) string {
	if g != nil {
		return "OpenKart shows " + g.Name + " time-trial rankings and ghosts from OpenPak."
	}
	return "OpenKart shows rankings, ghosts and tournaments for Mario Kart 8, 8 Deluxe, 7 and Wii from OpenPak."
}

func pageParam(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || n < 1 || n > 10000 {
		return 1
	}
	return n
}

// Pager is a previous/next pair for a list.
type Pager struct {
	Page, Pages int
	Prev, Next  string
}

func pager(page, total int, base string) Pager {
	pages := (total + pageSize - 1) / pageSize
	if pages < 1 {
		pages = 1
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	p := Pager{Page: page, Pages: pages}
	if page > 1 {
		p.Prev = fmt.Sprintf("%s%spage=%d", base, sep, page-1)
	}
	if page < pages {
		p.Next = fmt.Sprintf("%s%spage=%d", base, sep, page+1)
	}
	return p
}

var courseLine = regexp.MustCompile(`^\s*(\d+)\s+(.+?)\s*$`)

func parseCourses(s string) map[int]string {
	out := map[int]string{}
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if m := courseLine.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			out[n] = m[2]
		}
	}
	return out
}

func mine(p *Player, g *Game, id string) bool {
	if p == nil || g == nil || id == "" {
		return false
	}
	// Ids are the game's player id, or an OpenPak account id where the ranking came from openpak.org.
	return g.Identity(p) == id || (p.Account != "" && p.Account == id)
}

// who is how a player is named in a list: the visitor as "You (name)", others by the name the
// game stored, else by player id.
func who(p *Player, g *Game, id, name string) string {
	switch {
	case mine(p, g, id):
		return "You (" + p.Display() + ")"
	case name != "":
		return name
	}
	return "Player " + id
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

var ytID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// youtubeEmbed is the privacy-enhanced embed URL for a video id, or "" when the id is not a
// well-formed one (so nothing but an 11-character id ever reaches a src attribute).
func youtubeEmbed(id string) string {
	if !ytID.MatchString(id) {
		return ""
	}
	return "https://www.youtube-nocookie.com/embed/" + id
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
	return t.UTC().Format("2006-01-02")
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
