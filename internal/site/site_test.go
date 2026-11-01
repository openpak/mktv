package site

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	hlJSON  = `{"total":1,"offset":0,"limit":6,"items":[{"id":7,"owner":"1800000005","name":"Wild one","size":2048,"data_type":1,"tags":["a"],"thumbnail":8,"created":"2026-10-30T12:00:00Z","updated":"2026-10-30T12:00:00Z"}]}`
	hl2JSON = `{"id":7,"owner":"1800000005","name":"Wild one","size":2048,"data_type":1,"tags":[],"thumbnail":8,"youtube_id":"dQw4w9WgXcQ","created":"2026-10-30T12:00:00Z","updated":"2026-10-30T12:00:00Z"}`
)

// fakeAPI answers like the mario-kart-8 read API.
func fakeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	j := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) }
	}
	mux.HandleFunc("/api/v1/mktv", j(hlJSON))
	mux.HandleFunc("/api/v1/objects/7", j(hl2JSON))
	mux.HandleFunc("/api/v1/objects/9", j(strings.Replace(hl2JSON, `"youtube_id":"dQw4w9WgXcQ"`, `"youtube_id":"\"><script>"`, 1)))
	mux.HandleFunc("/api/v1/objects/10", j(strings.Replace(hl2JSON, `"youtube_id":"dQw4w9WgXcQ",`, ``, 1)))
	mux.HandleFunc("/api/v1/objects/8/body", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("\x89PNG"))
	})
	mux.HandleFunc("/api/v1/courses", j(`{"items":[{"course":4,"ranked":2,"ghosts":1}]}`))
	mux.HandleFunc("/api/v1/courses/4/rankings", j(`{"total":2,"offset":0,"limit":20,"items":[{"rank":1,"player":"1800000005","time_ms":80000,"time":"1:20.000","updated":"2026-10-30T12:00:00Z"},{"rank":2,"player":"1800000006","time_ms":90000,"time":"1:30.000","updated":"2026-10-30T12:00:00Z"}]}`))
	mux.HandleFunc("/api/v1/courses/4/ghosts", j(`{"total":1,"offset":0,"limit":10,"items":[{"id":3,"owner":"1800000006","size":900,"data_type":4,"tags":[],"created":"2026-10-30T12:00:00Z","updated":"2026-10-30T12:00:00Z"}]}`))
	mux.HandleFunc("/api/v1/players/1800000005/rankings", j(`{"items":[{"course":4,"rank":1,"of":2,"time":"1:20.000"}]}`))
	mux.HandleFunc("/api/v1/tournaments", j(`{"total":1,"offset":0,"limit":5,"items":[{"id":20000,"owner":"1800000005","name":"Cup <b>","community_code":"111122223333","participants":2,"season":1,"created":"2026-10-30T12:00:00Z"}]}`))
	mux.HandleFunc("/api/v1/tournaments/20000", j(`{"id":20000,"owner":"1800000005","name":"Cup","community_code":"111122223333","participants":2,"season":2,"created":"2026-10-30T12:00:00Z","scores":[{"rank":1,"player":"1800000006","score":30,"updated":"2026-10-30T12:00:00Z"}]}`))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newSite(t *testing.T, core string) *Site {
	t.Helper()
	cfg := Config{Addr: ":0", APIURL: fakeAPI(t).URL, PublicURL: "https://mktv.test", OpenPakURL: "https://openpak.org", CoreURL: core, SessionKey: strings.Repeat("k", 32)}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func do(s http.Handler, method, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var r *http.Request
	if form != nil {
		r = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestPages(t *testing.T) {
	s := newSite(t, "")
	for path, want := range map[string]string{
		"/":                  "Wild one",
		"/highlights":        "Wild one",
		"/courses":           "Course 4",
		"/courses/4":         "1:30.000",
		"/tournaments":       "Cup &lt;b&gt;", // escaped
		"/tournaments/20000": "Leaderboard",
		"/about":             "Nothing outside the game can play it",
		"/healthz":           "ok mktv",
	} {
		w := do(s, "GET", path, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s: %d, missing %q", path, w.Code, want)
		}
	}
}

func TestShellMatchesOpenPak(t *testing.T) {
	body := do(newSite(t, ""), "GET", "/", nil).Body.String()
	for _, want := range []string{`class="top"`, `href="https://openpak.org/"`, `class="site-foot"`, `https://openpak.org/privacy`, `/static/base.css`} {
		if !strings.Contains(body, want) {
			t.Errorf("shell lacks %q", want)
		}
	}
}

func TestYouTubeSlot(t *testing.T) {
	s := newSite(t, "")
	with := do(s, "GET", "/highlights/7", nil).Body.String()
	if !strings.Contains(with, `src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"`) {
		t.Error("a stored id must embed")
	}
	if strings.Contains(with, "cannot be played in a browser") {
		t.Error("with a video the replay caveat is not needed")
	}
	without := do(s, "GET", "/highlights/10", nil).Body.String()
	if strings.Contains(without, "<iframe") || !strings.Contains(without, "/media/8") {
		t.Error("no id: no iframe, thumbnail instead")
	}
	bad := do(s, "GET", "/highlights/9", nil).Body.String()
	if strings.Contains(bad, "<iframe") || strings.Contains(bad, "<script>") {
		t.Error("a malformed id must never reach the page")
	}
}

func TestMediaAndNotFound(t *testing.T) {
	s := newSite(t, "")
	w := do(s, "GET", "/media/8", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("media: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	for _, p := range []string{"/media/99", "/media/x", "/highlights/x", "/courses/abc", "/tournaments/1?season=0", "/nope"} {
		if c := do(s, "GET", p, nil).Code; c != 404 {
			t.Errorf("%s: %d", p, c)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := do(newSite(t, ""), "GET", "/", nil).Header()
	if !strings.Contains(h.Get("Content-Security-Policy"), "default-src 'self'") || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers: %v", h)
	}
}

func TestUpstreamDown(t *testing.T) {
	s := newSite(t, "")
	s.api.base = "http://127.0.0.1:1"
	if c := do(s, "GET", "/", nil).Code; c != 503 {
		t.Fatalf("code %d", c)
	}
}

// fakeCore stands in for openpak.org's public API.
func fakeCore(t *testing.T, twoFactor, noWiiU bool) (*httptest.Server, *int) {
	t.Helper()
	revoked := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/token", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Email, Password string }
		_ = jsonDecode(r, &in)
		switch {
		case in.Password != "hunter2":
			w.WriteHeader(401)
		case twoFactor:
			w.WriteHeader(202)
			w.Write([]byte(`{"two_factor_required":true,"challenge":"ch1"}`))
		default:
			w.WriteHeader(201)
			w.Write([]byte(`{"token":"tok"}`))
		}
	})
	mux.HandleFunc("POST /api/v1/token/2fa", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Challenge, Code string }
		_ = jsonDecode(r, &in)
		if in.Challenge != "ch1" || in.Code != "123456" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"token":"tok"}`))
	})
	mux.HandleFunc("GET /api/v1/me/wiiu", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		if noWiiU {
			w.WriteHeader(404)
			return
		}
		w.Write([]byte(`{"pid":1800000005,"username":"mario","mii_name":"Mario","nex_password":"secret"}`))
	})
	mux.HandleFunc("DELETE /api/v1/token", func(w http.ResponseWriter, r *http.Request) { revoked++; w.WriteHeader(204) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &revoked
}

func TestSignInFlow(t *testing.T) {
	core, revoked := fakeCore(t, false, false)
	s := newSite(t, core.URL)
	if w := do(s, "POST", "/signin", url.Values{"email": {"a@b.co"}, "password": {"wrong"}}); w.Code != 401 || strings.Contains(w.Body.String(), "wrong") {
		t.Fatalf("bad password: %d", w.Code)
	}
	w := do(s, "POST", "/signin", url.Values{"email": {"a@b.co"}, "password": {"hunter2"}})
	if w.Code != 303 || *revoked != 1 {
		t.Fatalf("sign in: %d revoked %d", w.Code, *revoked)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie flags: %+v", cookie)
	}
	if strings.Contains(cookie.Value, "secret") || strings.Contains(cookie.Value, "hunter2") {
		t.Error("the cookie must carry no credential")
	}
	me := do(s, "GET", "/me", nil, cookie)
	if me.Code != 200 || !strings.Contains(me.Body.String(), "1:20.000") || !strings.Contains(me.Body.String(), "Mario") {
		t.Fatalf("me: %d %s", me.Code, me.Body)
	}
	if body := do(s, "GET", "/courses/4", nil, cookie).Body.String(); !strings.Contains(body, "You (Mario)") || !strings.Contains(body, "rank 1 of 2") {
		t.Error("signed in, the course page should mark the player")
	}
	// A tampered cookie is a stranger.
	forged := &http.Cookie{Name: cookie.Name, Value: strings.Replace(cookie.Value, ".", "x.", 1)}
	if c := do(s, "GET", "/me", nil, forged).Code; c != 303 {
		t.Errorf("forged cookie: %d", c)
	}
	if c := do(s, "GET", "/me", nil).Code; c != 303 {
		t.Errorf("anonymous /me: %d", c)
	}
	if out := do(s, "POST", "/signout", url.Values{}, cookie); out.Code != 303 || out.Result().Cookies()[0].MaxAge >= 0 {
		t.Error("sign out should clear the cookie")
	}
}

func TestSignInTwoFactorAndNoWiiU(t *testing.T) {
	core, revoked := fakeCore(t, true, false)
	s := newSite(t, core.URL)
	w := do(s, "POST", "/signin", url.Values{"email": {"a@b.co"}, "password": {"hunter2"}})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `name="challenge" value="ch1"`) {
		t.Fatalf("expected the code form: %d", w.Code)
	}
	if c := do(s, "POST", "/signin/2fa", url.Values{"challenge": {"ch1"}, "code": {"000000"}}).Code; c != 401 {
		t.Errorf("wrong code: %d", c)
	}
	if c := do(s, "POST", "/signin/2fa", url.Values{"challenge": {"ch1"}, "code": {"123456"}}).Code; c != 303 || *revoked != 1 {
		t.Errorf("right code: %d revoked %d", c, *revoked)
	}

	core2, revoked2 := fakeCore(t, false, true)
	s2 := newSite(t, core2.URL)
	w = do(s2, "POST", "/signin", url.Values{"email": {"a@b.co"}, "password": {"hunter2"}})
	if w.Code != 403 || *revoked2 != 1 {
		t.Errorf("no Wii U identity: %d revoked %d (the token must still be revoked)", w.Code, *revoked2)
	}
}

func TestSignInOff(t *testing.T) {
	s := newSite(t, "")
	if c := do(s, "GET", "/signin", nil).Code; c != 404 {
		t.Errorf("sign-in must not exist without a core URL: %d", c)
	}
}

func TestSignInRateLimit(t *testing.T) {
	core, _ := fakeCore(t, false, false)
	s := newSite(t, core.URL)
	var last int
	for i := 0; i < 12; i++ {
		last = do(s, "POST", "/signin", url.Values{"email": {"a@b.co"}, "password": {"x"}}).Code
	}
	if last != 429 {
		t.Errorf("after 10 attempts: %d", last)
	}
}

func TestConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if _, err := ConfigFromEnv(env(nil)); err == nil {
		t.Error("API URL is required")
	}
	if _, err := ConfigFromEnv(env(map[string]string{"MKTV_API_URL": "http://x:1", "MKTV_CORE_URL": "http://c", "MKTV_SESSION_KEY": "short"})); err == nil {
		t.Error("a short session key must be refused when sign-in is on")
	}
	if _, err := ConfigFromEnv(env(map[string]string{"MKTV_API_URL": "ftp://x"})); err == nil {
		t.Error("non-http URL")
	}
	c, err := ConfigFromEnv(env(map[string]string{"MKTV_API_URL": "http://x:1/"}))
	if err != nil || c.APIURL != "http://x:1" || c.Addr != ":20190" {
		t.Errorf("%+v %v", c, err)
	}
}

func TestParseCourses(t *testing.T) {
	m := parseCourses("# names\n4 Mario Kart Stadium\n  12   Toad Harbor  \nbad line\n")
	if m[4] != "Mario Kart Stadium" || m[12] != "Toad Harbor" || len(m) != 2 {
		t.Errorf("%v", m)
	}
}

func jsonDecode(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
