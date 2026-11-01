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
	"sync"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const pageSize = 20

// Site is the http.Handler of openkart.openpak.org.
type Site struct {
	cfg     Config
	api     *apiClient
	core    *coreClient
	courses map[int]string
	pages   map[string]*template.Template
	mux     *http.ServeMux
	asset   string

	limMu sync.Mutex
	lim   map[string][]time.Time
}

// New builds the site. It does not touch the network.
func New(cfg Config) (*Site, error) {
	s := &Site{cfg: cfg, api: newAPIClient(cfg.APIURL), courses: map[int]string{}, lim: map[string][]time.Time{},
		asset: strconv.FormatInt(time.Now().Unix(), 36)}
	if cfg.CoreURL != "" {
		s.core = newCoreClient(cfg.CoreURL)
	}
	if cfg.CoursesFile != "" {
		b, err := os.ReadFile(cfg.CoursesFile)
		if err != nil {
			return nil, err
		}
		s.courses = parseCourses(string(b))
	}
	funcs := template.FuncMap{
		"course":   s.courseName,
		"ago":      ago,
		"date":     func(t time.Time) string { return t.UTC().Format("2006-01-02") },
		"datetime": func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") },
		"bytes":    humanBytes,
		"yt":       youtubeEmbed,
		"add":      func(a int64, b int) int64 { return a + int64(b) },
		"mine":     func(p *Player, pid string) bool { return p != nil && p.PID == pid },
		"who":      func(p *Player, pid string) string { return who(p, pid) },
		"pages":    pager,
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
	s.mux.HandleFunc("GET /highlights", s.highlights)
	s.mux.HandleFunc("GET /highlights/{id}", s.highlight)
	s.mux.HandleFunc("GET /courses", s.courseList)
	s.mux.HandleFunc("GET /courses/{course}", s.courseView)
	s.mux.HandleFunc("GET /tournaments", s.tournaments)
	s.mux.HandleFunc("GET /tournaments/{id}", s.tournament)
	s.mux.HandleFunc("GET /media/{id}", s.media)
	s.mux.HandleFunc("GET /about", s.about)
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
	Err      string
	Flash    string
	Data     any
	NotFound bool
}

func (s *Site) render(w http.ResponseWriter, r *http.Request, status int, name, title string, data any) {
	v := view{Title: title, Path: r.URL.Path, Asset: s.asset, OpenPak: s.cfg.OpenPakURL, Contact: s.cfg.ContactEmail,
		SignIn: s.core != nil, Data: data}
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

// fail answers an upstream error: 404 for a missing thing, a plain 503 page otherwise.
func (s *Site) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errNotFound) {
		s.render(w, r, http.StatusNotFound, "error", "Not found", "That page does not exist, or the thing it showed is gone.")
		return
	}
	log.Printf("mktv: %s: %v", r.URL.Path, err)
	s.render(w, r, http.StatusServiceUnavailable, "error", "Unavailable", "Mario Kart 8's server did not answer. Try again in a minute.")
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

func (s *Site) courseName(n int) string {
	if name, ok := s.courses[n]; ok {
		return name
	}
	return "Course " + strconv.Itoa(n)
}

func who(p *Player, pid string) string {
	if p != nil && p.PID == pid {
		return "You (" + p.Display() + ")"
	}
	return "Player " + pid
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

// --- pages ---

type homeData struct {
	Highlights []Object
	Courses    []Course
	Tournament []Tournament
	Ghosts     int
	Stats      struct{ Highlights, Courses, Tournaments int }
}

func (s *Site) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hl, err := get[Page[Object]](ctx, s.api, "/api/v1/mktv?"+q(6, 0, ""))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	cs, err := get[struct{ Items []Course }](ctx, s.api, "/api/v1/courses")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ts, err := get[Page[Tournament]](ctx, s.api, "/api/v1/tournaments?"+q(5, 0, ""))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := homeData{Highlights: hl.Items, Courses: cs.Items, Tournament: ts.Items}
	d.Stats.Highlights, d.Stats.Courses, d.Stats.Tournaments = hl.Total, len(cs.Items), ts.Total
	s.render(w, r, 200, "home", "Home", d)
}

type listData struct {
	Items []Object
	Total int
	Pager Pager
}

func (s *Site) highlights(w http.ResponseWriter, r *http.Request) {
	pg := pageParam(r)
	p, err := get[Page[Object]](r.Context(), s.api, "/api/v1/mktv?"+q(pageSize, (pg-1)*pageSize, ""))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, 200, "highlights", "Highlights", listData{p.Items, p.Total, pager(pg, p.Total, "/highlights")})
}

func (s *Site) highlight(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, errNotFound)
		return
	}
	o, err := get[Object](r.Context(), s.api, fmt.Sprintf("/api/v1/objects/%d", id))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, 200, "highlight", fmt.Sprintf("Highlight %d", o.ID), o)
}

func (s *Site) courseList(w http.ResponseWriter, r *http.Request) {
	cs, err := get[struct{ Items []Course }](r.Context(), s.api, "/api/v1/courses")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, 200, "courses", "Courses", cs.Items)
}

type courseData struct {
	Course   int
	Ranks    Page[Rank]
	Ghosts   Page[Object]
	Pager    Pager
	Offset   int
	Standing *PlayerRank
}

func (s *Site) courseView(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("course"))
	if err != nil || n < 0 || n > 1<<20 {
		s.fail(w, r, errNotFound)
		return
	}
	pg := pageParam(r)
	ctx := r.Context()
	d := courseData{Course: n, Offset: (pg - 1) * pageSize}
	if d.Ranks, err = get[Page[Rank]](ctx, s.api, fmt.Sprintf("/api/v1/courses/%d/rankings?%s", n, q(pageSize, d.Offset, ""))); err != nil {
		s.fail(w, r, err)
		return
	}
	if d.Ghosts, err = get[Page[Object]](ctx, s.api, fmt.Sprintf("/api/v1/courses/%d/ghosts?%s", n, q(10, 0, ""))); err != nil {
		s.fail(w, r, err)
		return
	}
	if p, ok := s.player(r); ok {
		if mine, err := get[struct{ Items []PlayerRank }](ctx, s.api, "/api/v1/players/"+p.PID+"/rankings"); err == nil {
			for i := range mine.Items {
				if mine.Items[i].Course == n {
					d.Standing = &mine.Items[i]
				}
			}
		}
	}
	d.Pager = pager(pg, d.Ranks.Total, fmt.Sprintf("/courses/%d", n))
	s.render(w, r, 200, "course", s.courseName(n), d)
}

func (s *Site) tournaments(w http.ResponseWriter, r *http.Request) {
	pg := pageParam(r)
	p, err := get[Page[Tournament]](r.Context(), s.api, "/api/v1/tournaments?"+q(pageSize, (pg-1)*pageSize, ""))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, 200, "tournaments", "Tournaments", struct {
		Items []Tournament
		Total int
		Pager Pager
	}{p.Items, p.Total, pager(pg, p.Total, "/tournaments")})
}

func (s *Site) tournament(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil {
		s.fail(w, r, errNotFound)
		return
	}
	path := fmt.Sprintf("/api/v1/tournaments/%d", id)
	if v := r.URL.Query().Get("season"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 1<<20 {
			s.fail(w, r, errNotFound)
			return
		}
		path += "?season=" + strconv.Itoa(n)
	}
	t, err := get[Tournament](r.Context(), s.api, path)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, 200, "tournament", t.Name, t)
}

// media serves a thumbnail or ghost file from the title's store. Only what the API lists is
// reachable, and the API decides the content type (images are images, the rest downloads).
func (s *Site) media(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rec := &lazyWriter{ResponseWriter: w}
	if err := s.api.media(r.Context(), id, rec); err != nil && !rec.wrote {
		if errors.Is(err, errNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("mktv: media %d: %v", id, err)
		http.Error(w, "unavailable", http.StatusBadGateway)
	}
}

type lazyWriter struct {
	http.ResponseWriter
	wrote bool
}

func (l *lazyWriter) Write(b []byte) (int, error) { l.wrote = true; return l.ResponseWriter.Write(b) }

func (s *Site) about(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, 200, "about", "About", nil)
}
