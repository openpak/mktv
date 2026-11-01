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
	s.render(w, r, nil, status, "error", "Sign-in", "Signing in did not finish. Go back and press Sign in to try again.")
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
		AccountID   string          `json:"account_id"`
		DisplayName string          `json:"display_name"`
		WiiUPID     json.RawMessage `json:"wiiu_pid"`
		N3DSPID     json.RawMessage `json:"n3ds_pid"`
		SwitchPID   json.RawMessage `json:"switch_pid"`
		WFCUserID   json.RawMessage `json:"wfc_user_id"`
	}
	if err := json.Unmarshal(body, &t); err != nil || t.AccountID == "" {
		return Player{}, errors.Join(errExchange, errors.New("unreadable answer"))
	}
	name := []rune(t.DisplayName)
	if len(name) > 64 {
		name = name[:64]
	}
	return Player{Account: t.AccountID, Name: string(name), PID: playerID(t.WiiUPID), N3DS: playerID(t.N3DSPID),
		SwitchPID: playerID(t.SwitchPID), WFCUserID: playerID(t.WFCUserID)}, nil
}

// playerID reads an optional id that may arrive as a JSON string or number (null, empty or
// anything that is not a positive decimal number means "not linked"). Ids are kept as text: a
// Switch principal id does not fit a float.
func playerID(raw json.RawMessage) string {
	v := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if v == "" || v == "null" || len(v) > 20 {
		return ""
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return ""
		}
	}
	if strings.Trim(v, "0") == "" {
		return ""
	}
	return v
}

// authLogout ends this site's session only and returns to the home page. openpak.org stays
// signed in, like leaving one Google app; the next Sign in may sign straight back in.
func (s *Site) authLogout(w http.ResponseWriter, r *http.Request) {
	if !s.signInOn() {
		http.NotFound(w, r)
		return
	}
	noStore(w)
	s.clearSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
