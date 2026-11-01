package site

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
)

type handler func(g *Game, w http.ResponseWriter, r *http.Request)

func (s *Site) bind(g *Game, h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h(g, w, r) }
}

// routes registers one game's pages. Mario Kart 8 keeps the original top-level URLs
// (/highlights, /courses, /tournaments); the other games live under /{slug}/.
func (s *Site) routes(g *Game) {
	b := g.Base
	s.mux.HandleFunc("GET "+g.Home, s.bind(g, s.gameHome))
	s.mux.HandleFunc("GET "+b+"/courses", s.bind(g, s.courseList))
	s.mux.HandleFunc("GET "+b+"/courses/{course}", s.bind(g, s.courseView))
	if g.Highlights {
		s.mux.HandleFunc("GET "+b+"/highlights", s.bind(g, s.highlights))
		s.mux.HandleFunc("GET "+b+"/highlights/{id}", s.bind(g, s.highlight))
	}
	if g.Ghosts {
		s.mux.HandleFunc("GET "+b+"/ghosts", s.bind(g, s.ghosts))
	}
	if g.Ghosts || g.Highlights {
		s.mux.HandleFunc("GET "+b+"/media/{id}", s.bind(g, s.media))
	}
	if g.Tournaments {
		s.mux.HandleFunc("GET "+b+"/tournaments", s.bind(g, s.tournaments))
		s.mux.HandleFunc("GET "+b+"/tournaments/{id}", s.bind(g, s.tournament))
	}
}

func (s *Site) game(slug string) *Game {
	for _, g := range s.games {
		if g.Slug == slug {
			return g
		}
	}
	return nil
}

// --- pages ---

// home introduces the games. It makes no upstream call, so it is up whatever the servers do.
func (s *Site) home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, nil, 200, "home", "Home", nil)
}

type gameHomeData struct {
	Highlights  []Object
	Courses     []Course
	Tournaments []Tournament
	Stats       struct{ Highlights, Ranked, Ghosts, Tournaments int }
}

func (s *Site) gameHome(g *Game, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var d gameHomeData
	cs, err := get[items[Course]](ctx, g.api, "/courses")
	if err != nil {
		s.fail(w, r, g, err)
		return
	}
	d.Courses = cs
	for _, c := range cs {
		if c.Ranked > 0 {
			d.Stats.Ranked++
		}
		d.Stats.Ghosts += c.Ghosts
	}
	if g.Highlights {
		hl, err := getList[Object](ctx, g.api, "/mktv?"+q(6, 0, ""))
		if err != nil {
			s.fail(w, r, g, err)
			return
		}
		d.Highlights, d.Stats.Highlights = hl.Items, hl.Total
	}
	if g.Tournaments {
		ts, err := getList[Tournament](ctx, g.api, "/tournaments?"+q(5, 0, ""))
		if err != nil {
			s.fail(w, r, g, err)
			return
		}
		d.Tournaments, d.Stats.Tournaments = ts.Items, ts.Total
	}
	s.render(w, r, g, 200, "gamehome", g.Name, d)
}

type listData struct {
	Items []Object
	Total int
	Pager Pager
}

func (s *Site) highlights(g *Game, w http.ResponseWriter, r *http.Request) {
	pg := pageParam(r)
	p, err := getList[Object](r.Context(), g.api, "/mktv?"+q(pageSize, (pg-1)*pageSize, ""))
	if err != nil {
		s.fail(w, r, g, err)
		return
	}
	s.render(w, r, g, 200, "highlights", g.Name+" highlights", listData{p.Items, p.Total, pager(pg, p.Total, g.Base+"/highlights")})
}

func (s *Site) highlight(g *Game, w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, g, errNotFound)
		return
	}
	o, err := get[Object](r.Context(), g.api, fmt.Sprintf("/objects/%d", id))
	if errors.Is(err, errUnsupported) {
		err = errNotFound
	}
	if err != nil {
		s.fail(w, r, g, err)
		return
	}
	s.render(w, r, g, 200, "highlight", fmt.Sprintf("Highlight %d", o.ID), o)
}

func (s *Site) courseList(g *Game, w http.ResponseWriter, r *http.Request) {
	cs, err := get[items[Course]](r.Context(), g.api, "/courses")
	if err != nil {
		s.fail(w, r, g, err)
		return
	}
	s.render(w, r, g, 200, "courses", g.Name+" courses", []Course(cs))
}

type courseData struct {
	Course   int
	Ranks    Page[Rank]
	Ghosts   Page[Object]
	Pager    Pager
	Offset   int
	Standing *PlayerRank
}

func (s *Site) courseView(g *Game, w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("course"))
	if err != nil || n < 0 || n > 1<<20 {
		s.fail(w, r, g, errNotFound)
		return
	}
	pg := pageParam(r)
	ctx := r.Context()
	d := courseData{Course: n, Offset: (pg - 1) * pageSize}
	if d.Ranks, err = getList[Rank](ctx, g.api, fmt.Sprintf("/courses/%d/rankings?%s", n, q(pageSize, d.Offset, ""))); err != nil {
		s.fail(w, r, g, err)
		return
	}
	if g.Ghosts {
		if d.Ghosts, err = getList[Object](ctx, g.api, fmt.Sprintf("/courses/%d/ghosts?%s", n, q(10, 0, ""))); err != nil {
			s.fail(w, r, g, err)
			return
		}
	}
	if p, ok := s.player(r); ok {
		if id := g.Identity(&p); id != "" {
			if mine, err := get[items[PlayerRank]](ctx, g.api, "/players/"+id+"/rankings"); err == nil {
				for i := range mine {
					if mine[i].Course == n {
						d.Standing = &mine[i]
					}
				}
			}
		}
	}
	d.Pager = pager(pg, d.Ranks.Total, fmt.Sprintf("%s/courses/%d", g.Base, n))
	s.render(w, r, g, 200, "course", g.CourseName(n), d)
}

type ghostGroup struct {
	Course int
	Total  int
	Items  []Object
}

// ghosts lists the uploaded ghosts, course by course: the API lists ghosts per course, so this
// asks for each course that has any (in parallel; the client caches).
func (s *Site) ghosts(g *Game, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cs, err := get[items[Course]](ctx, g.api, "/courses")
	if err != nil {
		s.fail(w, r, g, err)
		return
	}
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		groups []ghostGroup
		failed error
	)
	sem := make(chan struct{}, 8)
	for _, c := range cs {
		if c.Ghosts == 0 {
			continue
		}
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p, err := getList[Object](ctx, g.api, fmt.Sprintf("/courses/%d/ghosts?%s", n, q(10, 0, "")))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed = err
				return
			}
			if len(p.Items) > 0 {
				groups = append(groups, ghostGroup{n, p.Total, p.Items})
			}
		}(c.Course)
	}
	wg.Wait()
	if failed != nil {
		s.fail(w, r, g, failed)
		return
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Course < groups[j].Course })
	s.render(w, r, g, 200, "ghosts", g.Name+" ghosts", groups)
}

func (s *Site) tournaments(g *Game, w http.ResponseWriter, r *http.Request) {
	pg := pageParam(r)
	p, err := getList[Tournament](r.Context(), g.api, "/tournaments?"+q(pageSize, (pg-1)*pageSize, ""))
	if err != nil {
		s.fail(w, r, g, err)
		return
	}
	s.render(w, r, g, 200, "tournaments", g.Name+" "+lower(g.TournamentsLabel), struct {
		Items []Tournament
		Total int
		Pager Pager
	}{p.Items, p.Total, pager(pg, p.Total, g.Base+"/tournaments")})
}

func lower(s string) string {
	if s == "Communities" {
		return "communities"
	}
	return "tournaments"
}

func (s *Site) tournament(g *Game, w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil {
		s.fail(w, r, g, errNotFound)
		return
	}
	path := fmt.Sprintf("/tournaments/%d", id)
	if v := r.URL.Query().Get("season"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 1<<20 {
			s.fail(w, r, g, errNotFound)
			return
		}
		path += "?season=" + strconv.Itoa(n)
	}
	t, err := get[Tournament](r.Context(), g.api, path)
	if errors.Is(err, errUnsupported) {
		err = errNotFound
	}
	if err != nil {
		s.fail(w, r, g, err)
		return
	}
	s.render(w, r, g, 200, "tournament", t.Name, t)
}

// media serves a thumbnail or ghost file from the game's store. Only what the API lists is
// reachable; images are images and everything else is a download.
func (s *Site) media(g *Game, w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, g, errNotFound)
		return
	}
	rec := &lazyWriter{ResponseWriter: w}
	if err := g.api.media(r.Context(), id, rec); err != nil && !rec.wrote {
		if errors.Is(err, errNotFound) {
			s.fail(w, r, g, errNotFound)
			return
		}
		log.Printf("mktv: media %d: %v", id, err)
		s.fail(w, r, g, err)
	}
}

type lazyWriter struct {
	http.ResponseWriter
	wrote bool
}

func (l *lazyWriter) Write(b []byte) (int, error) { l.wrote = true; return l.ResponseWriter.Write(b) }

func (s *Site) about(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, nil, 200, "about", "About", nil)
}

// --- your results ---

type meSection struct {
	Game       *Game
	ID         string // the visitor's player id for the game; "" = no console linked
	Ranks      []PlayerRank
	Highlights []Object
	Down       bool // the game's server did not answer
}

// me is "Your results": one section per offered game, by the player id the account carries for it.
func (s *Site) me(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(r)
	if !ok {
		if !s.signInOn() {
			s.fail(w, r, nil, errNotFound)
			return
		}
		http.Redirect(w, r, "/auth/login?next=/me", http.StatusSeeOther)
		return
	}
	secs := make([]meSection, len(s.games))
	var wg sync.WaitGroup
	for i, g := range s.games {
		secs[i] = meSection{Game: g, ID: g.Identity(&p)}
		if secs[i].ID == "" {
			continue
		}
		wg.Add(1)
		go func(sec *meSection) {
			defer wg.Done()
			g := sec.Game
			rk, err := get[items[PlayerRank]](r.Context(), g.api, "/players/"+sec.ID+"/rankings")
			if errors.Is(err, errNotFound) || errors.Is(err, errUnsupported) {
				err = nil
			}
			if err != nil {
				log.Printf("mktv: me %s: %v", g.Slug, err)
				sec.Down = true
				return
			}
			sec.Ranks = rk
			if g.Highlights {
				hl, err := getList[Object](r.Context(), g.api, "/mktv?"+q(20, 0, sec.ID))
				if err != nil {
					log.Printf("mktv: me %s: %v", g.Slug, err)
					sec.Down = true
					return
				}
				sec.Highlights = hl.Items
			}
		}(&secs[i])
	}
	wg.Wait()
	s.render(w, r, nil, 200, "me", "Your results", secs)
}
