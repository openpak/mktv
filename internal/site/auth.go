package site

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type signinData struct {
	Email     string
	Challenge string
	Err       string // shown above the form; the password is never echoed
}

func (s *Site) signinForm(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, 200, "signin", "Sign in", signinData{})
}

// allow is a small per-client limiter on top of the account service's own.
func (s *Site) allow(r *http.Request) bool {
	key := r.Header.Get("CF-Connecting-IP")
	if key == "" {
		key = r.RemoteAddr
	}
	s.limMu.Lock()
	defer s.limMu.Unlock()
	now := time.Now()
	keep := s.lim[key][:0]
	for _, t := range s.lim[key] {
		if now.Sub(t) < 10*time.Minute {
			keep = append(keep, t)
		}
	}
	if len(keep) >= 10 {
		s.lim[key] = keep
		return false
	}
	s.lim[key] = append(keep, now)
	if len(s.lim) > 4096 {
		s.lim = map[string][]time.Time{}
	}
	return true
}

func (s *Site) signin(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email, password := strings.TrimSpace(r.PostFormValue("email")), r.PostFormValue("password")
	d := signinData{Email: email}
	if !s.allow(r) {
		s.signinFail(w, r, http.StatusTooManyRequests, d, "Too many attempts. Try again in a few minutes.")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil || password == "" {
		s.signinFail(w, r, http.StatusBadRequest, d, "Enter the email and password of your OpenPak account.")
		return
	}
	p, err := s.core.login(r.Context(), email, password)
	s.signinResult(w, r, d, p, err)
}

func (s *Site) signin2FA(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	d := signinData{Challenge: r.PostFormValue("challenge")}
	if !s.allow(r) {
		s.signinFail(w, r, http.StatusTooManyRequests, d, "Too many attempts. Try again in a few minutes.")
		return
	}
	code := strings.TrimSpace(r.PostFormValue("code"))
	if d.Challenge == "" || code == "" {
		s.signinFail(w, r, http.StatusBadRequest, d, "Enter the code from your authenticator.")
		return
	}
	p, err := s.core.login2FA(r.Context(), d.Challenge, code)
	s.signinResult(w, r, d, p, err)
}

func (s *Site) signinResult(w http.ResponseWriter, r *http.Request, d signinData, p Player, err error) {
	var ch twoFactorChallenge
	switch {
	case err == nil:
		s.setSession(w, p)
		http.Redirect(w, r, "/me", http.StatusSeeOther)
	case errors.As(err, &ch):
		d.Challenge = ch.Challenge
		s.render(w, r, 200, "signin", "Sign in", d)
	case errors.Is(err, errBadLogin):
		s.signinFail(w, r, http.StatusUnauthorized, d, "That email and password (or code) did not work.")
	case errors.Is(err, errNoWiiU):
		s.signinFail(w, r, http.StatusForbidden, d, "This OpenPak account has no Wii U identity yet. Link a Wii U on openpak.org first.")
	default:
		s.signinFail(w, r, http.StatusBadGateway, d, "OpenPak's account service did not answer. Try again in a minute.")
	}
}

func (s *Site) signinFail(w http.ResponseWriter, r *http.Request, status int, d signinData, msg string) {
	d.Err = msg
	s.render(w, r, status, "signin", "Sign in", d)
}

func (s *Site) signout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type meData struct {
	Ranks      []PlayerRank
	Highlights Page[Object]
}

func (s *Site) me(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(r)
	if !ok {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
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
