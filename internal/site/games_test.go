package site

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// signIn runs the whole single sign-on flow against the fake and returns the session cookie.
func signIn(t *testing.T, s *Site, f *fakeSSO) *http.Cookie {
	t.Helper()
	u, flowC := startLogin(t, s, f, "/auth/login")
	w := do(s, "GET", "/auth/callback?code=good&state="+url.QueryEscape(u.Query().Get("state")), nil, flowC)
	if w.Code != 303 {
		t.Fatalf("callback: %d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	t.Fatal("no session")
	return nil
}

var allPages = []string{
	"/", "/about", "/mk8", "/highlights", "/highlights/7", "/courses", "/courses/4", "/ghosts", "/tournaments", "/tournaments/20000",
	"/mk8d", "/mk8d/courses", "/mk8d/courses/4", "/mk8d/ghosts", "/mk8d/tournaments", "/mk8d/tournaments/20000",
	"/mk7", "/mk7/courses", "/mk7/courses/4", "/mk7/ghosts", "/mk7/tournaments", "/mk7/tournaments/20000",
	"/mkw", "/mkw/courses", "/mkw/courses/4", "/mkw/ghosts",
}

func TestEveryGamePage(t *testing.T) {
	s := newSite(t, "")
	for _, p := range allPages {
		w := do(s, "GET", p, nil)
		if w.Code != 200 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
	// Mario Kart Wii goes through its own prefix and key (the fake refuses calls without it).
	if b := do(s, "GET", "/mkw/courses/8", nil).Body.String(); !strings.Contains(b, "Luigi Circuit") {
		t.Error("the Wii course 8 is Luigi Circuit")
	}
	if b := do(s, "GET", "/mk8d/courses/4", nil).Body.String(); !strings.Contains(b, "Course 4") || strings.Contains(b, "Mario Kart Stadium") {
		t.Error("an undocumented mapping must stay 'Course N'")
	}
	for _, p := range []string{"/mkw/highlights", "/mk7/highlights", "/mkw/tournaments"} {
		if c := do(s, "GET", p, nil).Code; c != 404 {
			t.Errorf("%s: %d, a game offers only what it has", p, c)
		}
	}
	if b := do(s, "GET", "/mk7/tournaments", nil).Body.String(); !strings.Contains(b, "Communities") {
		t.Error("Mario Kart 7 calls them communities")
	}
}

func TestSwitcherOffersConfiguredGamesOnly(t *testing.T) {
	cfg := testConfig(t)
	cfg.MK7URL, cfg.MKWURL = "", ""
	cfg.PublicURL = "https://mktv.test"
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b := do(s, "GET", "/", nil).Body.String()
	for _, want := range []string{`href="/mk8"`, `href="/mk8d"`} {
		if !strings.Contains(b, want) {
			t.Errorf("home lacks %s", want)
		}
	}
	for _, not := range []string{`href="/mk7"`, `href="/mkw"`, "Mario Kart Wii", "Mario Kart 7"} {
		if strings.Contains(b, not) {
			t.Errorf("an unconfigured game must not be offered: %s", not)
		}
	}
	if c := do(s, "GET", "/mkw/courses", nil).Code; c != 404 {
		t.Errorf("/mkw/courses: %d", c)
	}
	if c := do(s, "GET", "/mk7", nil).Code; c != 404 {
		t.Errorf("/mk7: %d", c)
	}
	if _, err := New(Config{}); err == nil {
		t.Error("a site with no game is a mistake")
	}
}

func TestSwitcherMarksCurrentGame(t *testing.T) {
	b := do(newSite(t, ""), "GET", "/mk8d/courses", nil).Body.String()
	if !strings.Contains(b, `<a href="/mk8d" title="Mario Kart 8 Deluxe" aria-current="page">Switch</a>`) ||
		!strings.Contains(b, `<a href="/mk8d/courses" aria-current="page">Courses</a>`) ||
		!strings.Contains(b, `href="/mk8d/ghosts"`) {
		t.Errorf("switcher: %s", b)
	}
}

func TestGameHomeCounts(t *testing.T) {
	b := do(newSite(t, ""), "GET", "/mk8d", nil).Body.String()
	if !strings.Contains(b, "<b>1</b><span>of 96 courses with times") || !strings.Contains(b, "<b>1</b><span>ghost</span>") {
		t.Errorf("counts: %s", b)
	}
}

func TestGhostDownloadIsAnAttachment(t *testing.T) {
	s := newSite(t, "")
	for _, p := range []string{"/media/3", "/mk8d/media/3", "/mk7/media/3", "/mkw/media/3"} {
		w := do(s, "GET", p, nil)
		if w.Code != 200 || w.Header().Get("Content-Type") != "application/octet-stream" ||
			!strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") {
			t.Errorf("%s: %d %v", p, w.Code, w.Header())
		}
	}
	if b := do(s, "GET", "/mk8d/ghosts", nil).Body.String(); !strings.Contains(b, `href="/mk8d/media/3"`) || !strings.Contains(b, "Download ghost") {
		t.Error("the ghost list links the download")
	}
}

// emptyAPI answers every list as empty, like a server nobody has played on yet.
func emptyAPI(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/courses"):
			w.Write([]byte(`[]`)) // a bare list is allowed too
		case strings.Contains(r.URL.Path, "/objects/"), strings.Contains(r.URL.Path, "/tournaments/"):
			w.WriteHeader(404)
		case strings.HasSuffix(r.URL.Path, "/mktv"), strings.Contains(r.URL.Path, "/ghosts"):
			w.WriteHeader(501) // no object store
			w.Write([]byte(`{"error":"no object store"}`))
		default:
			w.Write([]byte(`{"total":0,"offset":0,"limit":20,"items":[]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestEmptyStates(t *testing.T) {
	cfg := Config{APIURL: emptyAPI(t), MK8DURL: emptyAPI(t), MK7URL: emptyAPI(t), MKWURL: emptyAPI(t), CoursesFiles: map[string]string{}}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/mk8", "/highlights", "/courses", "/courses/4", "/ghosts", "/tournaments", "/mk8d", "/mk8d/courses", "/mk8d/courses/1", "/mk8d/ghosts",
		"/mk8d/tournaments", "/mk7", "/mk7/ghosts", "/mk7/tournaments", "/mkw", "/mkw/courses", "/mkw/courses/3", "/mkw/ghosts"} {
		w := do(s, "GET", p, nil)
		if w.Code != 200 {
			t.Errorf("%s: %d", p, w.Code)
			continue
		}
		if !strings.Contains(w.Body.String(), `class="card empty-card"`) {
			t.Errorf("%s: no empty state", p)
		}
	}
	// A missing single thing is a 404 page, not a stack.
	if c := do(s, "GET", "/tournaments/5", nil).Code; c != 404 {
		t.Errorf("missing tournament: %d", c)
	}
}

func TestNoLeaksInAnyPage(t *testing.T) {
	f := newFakeSSO(t)
	f.extra = `,"switch_pid":"12297829382472990721","wfc_user_id":8899001122`
	s := newSSOSite(t, f)
	sess := signIn(t, s, f)
	pages := append([]string{"/me", "/nope", "/mk8d/nope"}, allPages...)
	banned := []string{"pretendo", "Pretendo", mkwKey, "s3cret", strings.Repeat("k", 32), "NEX", "BOSS", "gRPC", "DataStore", "{{", "<no value>", "TODO", "undefined", "%!", "panic", "goroutine", "X-WFC", "/api/"}
	for _, p := range pages {
		for _, c := range []*http.Cookie{nil, sess} {
			var w *httptest.ResponseRecorder
			if c == nil {
				w = do(s, "GET", p, nil)
			} else {
				w = do(s, "GET", p, nil, c)
			}
			if w.Code == 303 {
				continue
			}
			body := w.Body.String()
			for _, b := range banned {
				if strings.Contains(body, b) {
					t.Errorf("%s contains %q", p, b)
				}
			}
			for _, must := range []string{`class="top"`, `class="site-foot"`, "Part of the OpenPak network"} {
				if !strings.Contains(body, must) {
					t.Errorf("%s lacks %q", p, must)
				}
			}
		}
	}
}

func TestYourResultsPerGame(t *testing.T) {
	f := newFakeSSO(t)
	f.extra = `,"switch_pid":"12297829382472990721","wfc_user_id":8899001122`
	s := newSSOSite(t, f)
	sess := signIn(t, s, f)
	if len(sess.Value) > 400 {
		t.Errorf("session cookie is %d bytes", len(sess.Value))
	}
	b := do(s, "GET", "/me", nil, sess).Body.String()
	for _, h := range []string{"Mario Kart 8 <small", "Mario Kart 8 Deluxe <small", "Mario Kart 7 <small", "Mario Kart Wii <small"} {
		if !strings.Contains(b, h) {
			t.Errorf("no section %q", h)
		}
	}
	if n := strings.Count(b, "1:20.000"); n != 4 {
		t.Errorf("each of the four games shows the player's time, got %d", n)
	}
	if strings.Contains(b, "no Wii U linked") || strings.Contains(b, "linked, so there are no") {
		t.Errorf("everything is linked: %s", b)
	}
	// The 3DS uses the Wii U PID (docs/sso.md: one PNID for both); Mario Kart Wii a wfc id; the Switch a decimal string.
	for path, want := range map[string]string{"/courses/4": "rank 1 of 2", "/mk8d/courses/4": "rank 1 of 2", "/mk7/courses/4": "rank 1 of 2", "/mkw/courses/4": "rank 1 of 2"} {
		if !strings.Contains(do(s, "GET", path, nil, sess).Body.String(), want) {
			t.Errorf("%s: no standing", path)
		}
	}
}

func TestYourResultsUnlinkedGames(t *testing.T) {
	f := newFakeSSO(t)
	f.pid = "null"
	s := newSSOSite(t, f)
	b := do(s, "GET", "/me", nil, signIn(t, s, f)).Body.String()
	for _, want := range []string{"no Wii U linked", "no Switch linked", "no 3DS linked", "no Wii linked", `href="https://openpak.org/account"`} {
		if !strings.Contains(b, want) {
			t.Errorf("lacks %q", want)
		}
	}
	if strings.Contains(b, "1:20.000") {
		t.Error("no results without a link")
	}
}

func TestOnlySomeGamesLinked(t *testing.T) {
	f := newFakeSSO(t)
	f.pid = "null"
	f.extra = `,"switch_pid":"12297829382472990721"`
	s := newSSOSite(t, f)
	b := do(s, "GET", "/me", nil, signIn(t, s, f)).Body.String()
	if strings.Count(b, "1:20.000") != 1 || !strings.Contains(b, "no Wii U linked") || !strings.Contains(b, "no 3DS linked") || strings.Contains(b, "no Switch linked") {
		t.Errorf("only the Switch is linked: %s", b)
	}
}

func TestOwn3DSIDWins(t *testing.T) {
	f := newFakeSSO(t)
	f.extra = `,"n3ds_pid":1800000099`
	s := newSSOSite(t, f)
	sess := signIn(t, s, f)
	var p Player
	if !s.open(sess.Value, &p) || p.PID != "1800000005" || p.N3DSPID() != "1800000099" {
		t.Errorf("player: %+v", p)
	}
	b := do(s, "GET", "/me", nil, sess).Body.String()
	if !strings.Contains(b, "You have no time trial on the server yet.") { // 1800000099 has no times in the fake
		t.Errorf("3DS must use its own id: %s", b)
	}
}

func TestPlayerID(t *testing.T) {
	for in, want := range map[string]string{`"12297829382472990721"`: "12297829382472990721", `1800000005`: "1800000005", `null`: "", ``: "", `"abc"`: "", `0`: "", `"-5"`: "", `1.5`: "", `"123456789012345678901"`: ""} {
		if got := playerID([]byte(in)); got != want {
			t.Errorf("playerID(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestSessionCookieSize(t *testing.T) {
	s := newSite(t, "x")
	p := Player{Account: "6f1c2a3e-1111-4222-8333-444455556666", Name: strings.Repeat("é", 64), PID: "1800000005", N3DS: "1800000099",
		SwitchPID: "12297829382472990721", WFCUserID: "8899001122", Expires: 1 << 40}
	if v := s.seal(p); len(v) > 600 {
		t.Errorf("worst-case session cookie is %d bytes", len(v))
	}
}

func TestCourseTables(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range mkwCourses {
		if n == "" || seen[n] {
			t.Errorf("empty or duplicate course %q", n)
		}
		seen[n] = true
	}
	counts := map[string]int{"mk8": 48, "mk8d": 96, "mk7": 32, "mkw": 32}
	for _, d := range gameDefs() {
		g := d.Game
		if g.Count != counts[g.Slug] {
			t.Errorf("%s has %d courses, want %d", g.Slug, g.Count, counts[g.Slug])
		}
		if len(g.Names) != 0 && len(g.Names) != g.Count {
			t.Errorf("%s: %d names for %d courses", g.Slug, len(g.Names), g.Count)
		}
		if g.Slug != "mkw" && g.CourseName(3) != "Course 3" {
			t.Errorf("%s: undocumented ids stay 'Course N'", g.Slug)
		}
	}
	mkw := gameDefs()[3].Game
	if mkw.CourseName(0) != "Mario Circuit" || mkw.CourseName(31) != "GBA Shy Guy Beach" || mkw.CourseName(32) != "Course 32" {
		t.Error("Mario Kart Wii table")
	}
}

func TestConfigGames(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	c, err := ConfigFromEnv(env(map[string]string{"MKTV_MKW_API_URL": "http://race:20208/", "MKTV_MKW_API_KEY": "k", "MKTV_MK7_API_URL": "http://mk7:22855", "MKTV_MK8D_COURSES_FILE": "/x"}))
	if err != nil || c.MKWURL != "http://race:20208" || c.MKWKey != "k" || c.MK7URL != "http://mk7:22855" || c.APIURL != "" || c.CoursesFiles["mk8d"] != "/x" {
		t.Errorf("%+v %v", c, err)
	}
	if _, err := ConfigFromEnv(env(map[string]string{"MKTV_MK8D_API_URL": "ftp://x"})); err == nil {
		t.Error("bad scheme")
	}
}
