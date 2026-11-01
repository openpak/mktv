package site

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Sign-in is openpak.org's single sign-on (platforms/core/infra/website/docs/sso.md): the
// browser goes to openpak.org to sign in, comes back with a one-minute code, and this server
// swaps the code for the account behind it. The site never sees a password.

const ssoClient = "openkart"

func (s *Site) signInOn() bool {
	return s.cfg.CoreURL != "" && s.cfg.SSOSecret != "" && s.cfg.SessionKey != ""
}

func (s *Site) callbackURL() string { return s.cfg.PublicURL + "/auth/callback" }

// flow is what the short-lived cookie remembers between /auth/login and /auth/callback.
type flow struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
	Next     string `json:"n"`
	Expires  int64  `json:"e"`
}

func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// localPath keeps "next" on this site: a path, never another host.
func localPath(p string) string {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || strings.HasPrefix(p, "/auth/") || strings.ContainsAny(p, "\r\n") {
		return "/"
	}
	return p
}

func noStore(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }

func (s *Site) authLogin(w http.ResponseWriter, r *http.Request) {
	if !s.signInOn() {
		http.NotFound(w, r)
		return
	}
	noStore(w)
	f := flow{State: random(24), Verifier: random(32), Next: localPath(r.URL.Query().Get("next")), Expires: time.Now().Add(flowTTL).Unix()}
	s.setCookie(w, flowCookie, s.seal(f), "/auth", int(flowTTL.Seconds()))
	sum := sha256.Sum256([]byte(f.Verifier))
	q := url.Values{
		"client":                {ssoClient},
		"redirect_uri":          {s.callbackURL()},
		"state":                 {f.State},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
	}
	http.Redirect(w, r, s.cfg.OpenPakURL+"/account/sso/authorize?"+q.Encode(), http.StatusFound)
}

func (s *Site) authCallback(w http.ResponseWriter, r *http.Request) {
	if !s.signInOn() {
		http.NotFound(w, r)
		return
	}
	noStore(w)
	var f flow
	c, err := r.Cookie(flowCookie)
	s.setCookie(w, flowCookie, "", "/auth", -1) // a flow is used once, whatever happens
	q := r.URL.Query()
	code, state := q.Get("code"), q.Get("state")
	if err != nil || !s.open(c.Value, &f) || time.Now().Unix() > f.Expires || f.State == "" || state == "" || state != f.State || code == "" {
		s.signInFailed(w, r, http.StatusBadRequest)
		return
	}
	p, err := s.exchange(r.Context(), code, f.Verifier)
	if err != nil {
		log.Printf("openkart: sign-in exchange: %v", err)
		s.signInFailed(w, r, http.StatusBadGateway)
		return
	}
	s.setSession(w, p)
	http.Redirect(w, r, localPath(f.Next), http.StatusSeeOther)
}

func (s *Site) signInFailed(w http.ResponseWriter, r *http.Request, status int) {
	s.render(w, r, status, "error", "Sign-in", "Signing in did not finish. Go back and press Sign in to try again.")
}

var errExchange = errors.New("token exchange refused")

// exchange swaps the one-time code for the account, server to server.
func (s *Site) exchange(ctx context.Context, code, verifier string) (Player, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	form := url.Values{"code": {code}, "redirect_uri": {s.callbackURL()}, "code_verifier": {verifier}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.CoreURL+"/internal/sso/token", strings.NewReader(form.Encode()))
	if err != nil {
		return Player{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(ssoClient, s.cfg.SSOSecret)
	resp, err := s.sso.Do(req)
	if err != nil {
		return Player{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return Player{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Player{}, errors.Join(errExchange, errors.New(resp.Status))
	}
	var t struct {
		AccountID   string      `json:"account_id"`
		DisplayName string      `json:"display_name"`
		WiiUPID     json.Number `json:"wiiu_pid"`
	}
	if err := json.Unmarshal(body, &t); err != nil || t.AccountID == "" {
		return Player{}, errors.Join(errExchange, errors.New("unreadable answer"))
	}
	p := Player{Account: t.AccountID, Name: t.DisplayName}
	if n, err := strconv.ParseInt(t.WiiUPID.String(), 10, 64); err == nil && n > 0 {
		p.PID = strconv.FormatInt(n, 10)
	}
	return p, nil
}

// authLogout ends this site's session, then hands the browser to openpak.org, which asks
// before it ends its own and sends the person back here.
func (s *Site) authLogout(w http.ResponseWriter, r *http.Request) {
	if !s.signInOn() {
		http.NotFound(w, r)
		return
	}
	noStore(w)
	_, was := s.player(r)
	s.clearSession(w)
	if !was {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, s.cfg.OpenPakURL+"/account/sso/logout?redirect_uri="+url.QueryEscape(s.cfg.PublicURL+"/"), http.StatusSeeOther)
}

type meData struct {
	Ranks      []PlayerRank
	Highlights Page[Object]
}

// me is "Your results": the signed-in player's times and highlights, by Wii U player ID.
func (s *Site) me(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(r)
	if !ok {
		http.Redirect(w, r, "/auth/login?next=/me", http.StatusSeeOther)
		return
	}
	if p.PID == "" {
		s.render(w, r, 200, "me", "Your results", meData{})
		return
	}
	ctx := r.Context()
	rk, err := get[struct{ Items []PlayerRank }](ctx, s.api, "/api/v1/players/"+p.PID+"/rankings")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	hl, err := get[Page[Object]](ctx, s.api, "/api/v1/mktv?"+q(20, 0, p.PID))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, 200, "me", "Your results", meData{rk.Items, hl})
}
