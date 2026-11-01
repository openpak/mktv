package site

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const cookieName = "mktv_session"
const sessionTTL = 7 * 24 * time.Hour

// Player is what the site remembers about a signed-in visitor: their Wii U PID and the names
// OpenPak shows for it. Never a password, token or NEX credential.
type Player struct {
	PID      string `json:"p"`
	Username string `json:"u"`
	MiiName  string `json:"m"`
	Expires  int64  `json:"e"`
}

func (p Player) Display() string {
	if p.MiiName != "" {
		return p.MiiName
	}
	if p.Username != "" {
		return p.Username
	}
	return "Player " + p.PID
}

func (s *Site) sign(data []byte) string {
	m := hmac.New(sha256.New, []byte(s.cfg.SessionKey))
	m.Write(data)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (s *Site) setSession(w http.ResponseWriter, p Player) {
	p.Expires = time.Now().Add(sessionTTL).Unix()
	b, _ := json.Marshal(p)
	v := base64.RawURLEncoding.EncodeToString(b) + "." + s.sign(b)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: v, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.cfg.PublicURL, "https://"),
		SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTTL.Seconds())})
}

func (s *Site) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
}

// player returns the signed-in visitor, if the cookie is ours and unexpired.
func (s *Site) player(r *http.Request) (Player, bool) {
	var p Player
	if s.cfg.CoreURL == "" {
		return p, false
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		return p, false
	}
	body, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return p, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || !hmac.Equal([]byte(sig), []byte(s.sign(raw))) {
		return p, false
	}
	if json.Unmarshal(raw, &p) != nil || p.PID == "" || time.Now().Unix() > p.Expires {
		return Player{}, false
	}
	return p, true
}
