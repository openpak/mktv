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

const (
	cookieName = "openkart_session"
	flowCookie = "openkart_sso"
	sessionTTL = 7 * 24 * time.Hour
	flowTTL    = 10 * time.Minute
)

// Player is what the site remembers about a signed-in visitor: the OpenPak account, the name
// openpak.org shows for it and the player ids of the consoles it has linked, one per game family.
// Never a password or token. A session is well under 400 bytes, far below the cookie limit.
type Player struct {
	Account   string `json:"a"`
	Name      string `json:"n"`
	PID       string `json:"p,omitempty"` // Wii U PID (also the 3DS PID unless N3DS is set)
	N3DS      string `json:"d,omitempty"` // 3DS PID, only when the answer carries one of its own
	SwitchPID string `json:"s,omitempty"` // Switch principal id, decimal
	WFCUserID string `json:"w,omitempty"` // Wii/DS console user id, decimal
	Expires   int64  `json:"e"`
}

// N3DSPID is the 3DS player id: openpak.org gives one PNID to a Wii U and a 3DS (sso.md), so
// without a 3DS id of its own the Wii U PID is the 3DS PID.
func (p Player) N3DSPID() string {
	if p.N3DS != "" {
		return p.N3DS
	}
	return p.PID
}

func (p Player) Display() string {
	if p.Name != "" {
		return p.Name
	}
	return "Player"
}

// Initial is the letter on the account chip, like openpak.org's.
func (p Player) Initial() string {
	for _, r := range p.Display() {
		return strings.ToUpper(string(r))
	}
	return "?"
}

func (s *Site) sign(data []byte) string {
	m := hmac.New(sha256.New, []byte(s.cfg.SessionKey))
	m.Write(data)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// seal serialises v into a tamper-proof cookie value; open is its inverse.
func (s *Site) seal(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b) + "." + s.sign(b)
}

func (s *Site) open(value string, v any) bool {
	body, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || !hmac.Equal([]byte(sig), []byte(s.sign(raw))) {
		return false
	}
	return json.Unmarshal(raw, v) == nil
}

func (s *Site) secure() bool { return strings.HasPrefix(s.cfg.PublicURL, "https://") }

func (s *Site) setCookie(w http.ResponseWriter, name, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: path, HttpOnly: true, Secure: s.secure(),
		SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}

func (s *Site) setSession(w http.ResponseWriter, p Player) {
	p.Expires = time.Now().Add(sessionTTL).Unix()
	s.setCookie(w, cookieName, s.seal(p), "/", int(sessionTTL.Seconds()))
}

func (s *Site) clearSession(w http.ResponseWriter) { s.setCookie(w, cookieName, "", "/", -1) }

// player returns the signed-in visitor, if the cookie is ours and unexpired.
func (s *Site) player(r *http.Request) (Player, bool) {
	var p Player
	if !s.signInOn() {
		return p, false
	}
	c, err := r.Cookie(cookieName)
	if err != nil || !s.open(c.Value, &p) || p.Account == "" || time.Now().Unix() > p.Expires {
		return Player{}, false
	}
	return p, true
}
