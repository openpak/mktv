package site

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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
	for _, want := range []string{`class="top"`, `href="https://openpak.org/"`, `class="site-foot"`, `https://openpak.org/privacy`, `/static/base.css`, `<a href="https://openpak.org/">Part of the OpenPak network</a>`} {
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

// fakeSSO stands in for openpak.org's /internal/sso/token. It checks the client credentials and
// the PKCE verifier like the real one and records what it was asked.
type fakeSSO struct {
	srv       *httptest.Server
	calls     int
	form      url.Values
	challenge string // set by the test from the authorize redirect
	pid       string // JSON for wiiu_pid
	fail      bool
}

func newFakeSSO(t *testing.T) *fakeSSO {
	t.Helper()
	f := &fakeSSO{pid: "1800000005"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls++
		_ = r.ParseForm()
		f.form = r.PostForm
		id, secret, _ := r.BasicAuth()
		sum := sha256.Sum256([]byte(r.PostFormValue("code_verifier")))
		if r.URL.Path != "/internal/sso/token" || r.Method != "POST" || id != "openkart" || secret != "s3cret" ||
			r.PostFormValue("code") != "good" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge || f.fail {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		fmt.Fprintf(w, `{"account_id":"acc-1","display_name":"Mario","wiiu_pid":%s}`, f.pid)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newSSOSite(t *testing.T, f *fakeSSO) *Site {
	t.Helper()
	cfg := Config{Addr: ":0", APIURL: fakeAPI(t).URL, PublicURL: "https://mktv.test", OpenPakURL: "https://openpak.org",
		CoreURL: f.srv.URL, SSOSecret: "s3cret", SessionKey: strings.Repeat("k", 32)}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// startLogin runs /auth/login and returns the authorize URL and the flow cookie.
func startLogin(t *testing.T, s *Site, f *fakeSSO, target string) (*url.URL, *http.Cookie) {
	t.Helper()
	w := do(s, "GET", target, nil)
	if w.Code != 302 {
		t.Fatalf("login: %d", w.Code)
	}
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	f.challenge = u.Query().Get("code_challenge")
	cs := w.Result().Cookies()
	if len(cs) != 1 {
		t.Fatalf("cookies: %v", cs)
	}
	return u, cs[0]
}

func TestSignInOff(t *testing.T) {
	s := newSite(t, "")
	for _, p := range []string{"/auth/login", "/auth/callback", "/auth/logout"} {
		if c := do(s, "GET", p, nil).Code; c != 404 {
			t.Errorf("%s must not exist without sign-in configured: %d", p, c)
		}
	}
	if body := do(s, "GET", "/", nil).Body.String(); strings.Contains(body, "Sign in") {
		t.Error("no Sign in button when sign-in is off")
	}
}

func TestAuthLoginRedirect(t *testing.T) {
	f := newFakeSSO(t)
	s := newSSOSite(t, f)
	u, c := startLogin(t, s, f, "/auth/login?next=/courses/4")
	q := u.Query()
	if u.Host != "openpak.org" || u.Path != "/account/sso/authorize" || q.Get("client") != "openkart" ||
		q.Get("redirect_uri") != "https://mktv.test/auth/callback" || q.Get("code_challenge_method") != "S256" ||
		len(q.Get("state")) < 8 || q.Get("code_challenge") == "" {
		t.Errorf("authorize URL: %s", u)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.MaxAge <= 0 || c.Path != "/auth" {
		t.Errorf("flow cookie: %+v", c)
	}
	if strings.Contains(c.Value, "s3cret") {
		t.Error("the secret must never reach a cookie")
	}
}

func TestCallbackHappyPath(t *testing.T) {
	f := newFakeSSO(t)
	s := newSSOSite(t, f)
	u, flowC := startLogin(t, s, f, "/auth/login?next=/courses/4")
	w := do(s, "GET", "/auth/callback?code=good&state="+url.QueryEscape(u.Query().Get("state")), nil, flowC)
	if w.Code != 303 || w.Header().Get("Location") != "/courses/4" {
		t.Fatalf("callback: %d %s", w.Code, w.Header().Get("Location"))
	}
	if f.calls != 1 || f.form.Get("redirect_uri") != "https://mktv.test/auth/callback" {
		t.Errorf("exchange: %d %v", f.calls, f.form)
	}
	var sess *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			sess = c
		}
	}
	if sess == nil || !sess.HttpOnly || !sess.Secure || sess.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie: %+v", sess)
	}
	if strings.Contains(sess.Value, "s3cret") {
		t.Error("no secret in the session")
	}
	// The header now shows the account chip and a Sign out link, and still no form.
	page := do(s, "GET", "/about", nil, sess).Body.String()
	if !strings.Contains(page, `href="/auth/logout"`) || !strings.Contains(page, `class="avatar-btn"`) || !strings.Contains(page, `title="Mario"`) || strings.Contains(page, "<form") {
		t.Errorf("signed-in header: %s", page)
	}
	me := do(s, "GET", "/me", nil, sess)
	if me.Code != 200 || !strings.Contains(me.Body.String(), "1:20.000") || !strings.Contains(me.Body.String(), "Mario") {
		t.Fatalf("me: %d %s", me.Code, me.Body)
	}
	if body := do(s, "GET", "/courses/4", nil, sess).Body.String(); !strings.Contains(body, "You (Mario)") || !strings.Contains(body, "rank 1 of 2") {
		t.Error("signed in, the course page should mark the player")
	}
	// A forged session is a stranger.
	forged := &http.Cookie{Name: cookieName, Value: strings.Replace(sess.Value, ".", "x.", 1)}
	if c := do(s, "GET", "/me", nil, forged).Code; c != 303 {
		t.Errorf("forged session: %d", c)
	}
	if c := do(s, "GET", "/me", nil).Code; c != 303 {
		t.Errorf("anonymous /me: %d", c)
	}
	// Sign out clears the session and goes through openpak.org.
	out := do(s, "GET", "/auth/logout", nil, sess)
	loc := out.Header().Get("Location")
	if out.Code != 303 || loc != "https://openpak.org/account/sso/logout?redirect_uri=https%3A%2F%2Fmktv.test%2F" {
		t.Errorf("logout: %d %s", out.Code, loc)
	}
	if cs := out.Result().Cookies(); len(cs) == 0 || cs[0].MaxAge >= 0 {
		t.Error("sign out should clear the session")
	}
}

func TestCallbackStateMismatch(t *testing.T) {
	f := newFakeSSO(t)
	s := newSSOSite(t, f)
	_, flowC := startLogin(t, s, f, "/auth/login")
	for name, req := range map[string]string{
		"wrong state":   "/auth/callback?code=good&state=nope",
		"missing state": "/auth/callback?code=good",
		"missing code":  "/auth/callback?state=x",
	} {
		w := do(s, "GET", req, nil, flowC)
		if w.Code != 400 || f.calls != 0 {
			t.Errorf("%s: %d, exchanges %d", name, w.Code, f.calls)
		}
		for _, c := range w.Result().Cookies() {
			if c.Name == cookieName {
				t.Errorf("%s: a session was started", name)
			}
		}
	}
	// No flow cookie at all (a callback someone made up) and a tampered one.
	if w := do(s, "GET", "/auth/callback?code=good&state=abc", nil); w.Code != 400 {
		t.Errorf("no flow cookie: %d", w.Code)
	}
	bad := &http.Cookie{Name: flowCookie, Value: "x" + flowC.Value}
	if w := do(s, "GET", "/auth/callback?code=good&state=abc", nil, bad); w.Code != 400 || f.calls != 0 {
		t.Errorf("tampered flow cookie: %d", w.Code)
	}
}

func TestCallbackExchangeFails(t *testing.T) {
	f := newFakeSSO(t)
	s := newSSOSite(t, f)
	u, flowC := startLogin(t, s, f, "/auth/login")
	state := url.QueryEscape(u.Query().Get("state"))
	w := do(s, "GET", "/auth/callback?code=bad&state="+state, nil, flowC)
	if w.Code != 502 || !strings.Contains(w.Body.String(), "Signing in did not finish") {
		t.Errorf("refused code: %d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			t.Error("a failed exchange must not start a session")
		}
	}
	s.cfg.CoreURL = "http://127.0.0.1:1"
	if w := do(s, "GET", "/auth/callback?code=good&state="+state, nil, flowC); w.Code != 502 {
		t.Errorf("unreachable website: %d", w.Code)
	}
}

func TestNoWiiULinked(t *testing.T) {
	f := newFakeSSO(t)
	f.pid = "null"
	s := newSSOSite(t, f)
	u, flowC := startLogin(t, s, f, "/auth/login")
	w := do(s, "GET", "/auth/callback?code=good&state="+url.QueryEscape(u.Query().Get("state")), nil, flowC)
	if w.Code != 303 {
		t.Fatalf("callback: %d", w.Code)
	}
	var sess *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			sess = c
		}
	}
	body := do(s, "GET", "/me", nil, sess).Body.String()
	if !strings.Contains(body, "no Wii U linked") || !strings.Contains(body, `href="https://openpak.org/link"`) || strings.Contains(body, "Your times") {
		t.Errorf("me without a Wii U: %s", body)
	}
}

func TestSignedOutHeader(t *testing.T) {
	f := newFakeSSO(t)
	body := do(newSSOSite(t, f), "GET", "/courses", nil).Body.String()
	if !strings.Contains(body, `class="btn ghost" href="/auth/login?next=%2fcourses"`) || strings.Contains(body, "<form") || strings.Contains(body, "assword") {
		t.Errorf("signed-out header: %s", body)
	}
}

func TestLocalPath(t *testing.T) {
	for in, want := range map[string]string{"/courses/4": "/courses/4", "//evil.test": "/", "https://evil.test": "/", "": "/", "/\\evil": "/", "/auth/logout": "/"} {
		if got := localPath(in); got != want {
			t.Errorf("localPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if _, err := ConfigFromEnv(env(nil)); err == nil {
		t.Error("API URL is required")
	}
	if _, err := ConfigFromEnv(env(map[string]string{"MKTV_API_URL": "http://x:1", "MKTV_CORE_URL": "http://c", "SSO_CLIENT_OPENKART_SECRET": "x", "MKTV_SESSION_KEY": "short"})); err == nil {
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
