// Package site is the Mario Kart TV OpenKart website: pages over the read API of the mario-kart-8 NEX
// title, with sign-in through openpak.org single sign-on.
package site

import (
	"fmt"
	"net/url"
	"strings"
)

// Config is read from the environment (see example.env).
type Config struct {
	Addr    string // MKTV_ADDR, default :20190
	APIURL  string // MKTV_API_URL (or MKTV_MK8_API_URL), the Mario Kart 8 (Wii U) read API
	MK8DURL string // MKTV_MK8D_API_URL, Mario Kart 8 Deluxe (Switch)
	MK7URL  string // MKTV_MK7_API_URL, Mario Kart 7 (3DS)
	MKWURL  string // MKTV_MKW_API_URL, Mario Kart Wii (wfc-race; paths /api/mkw/v1)
	MKWKey  string // MKTV_MKW_API_KEY, sent to wfc-race as X-WFC-Internal-Key
	// CoursesFiles maps a game slug to its optional "<id> <name>" file (MKTV_COURSES_FILE is mk8's).
	CoursesFiles map[string]string
	PublicURL    string // MKTV_PUBLIC_URL, https://kart.openpak.org
	OpenPakURL   string // MKTV_OPENPAK_URL, links to the main site, default https://openpak.org
	CoreURL      string // MKTV_CORE_URL, where openpak.org's website is reachable from here (server to server); empty turns sign-in off
	SSOSecret    string // SSO_CLIENT_OPENKART_SECRET, this site's single sign-on client secret; empty turns sign-in off
	SessionKey   string // MKTV_SESSION_KEY, 32+ characters, signs the session cookies
	ContactEmail string // MKTV_CONTACT_EMAIL for the footer
}

// ConfigFromEnv builds and validates a Config.
func ConfigFromEnv(get func(string) string) (Config, error) {
	c := Config{
		Addr:         or(get("MKTV_ADDR"), ":20190"),
		APIURL:       strings.TrimRight(get("MKTV_API_URL"), "/"),
		PublicURL:    strings.TrimRight(or(get("MKTV_PUBLIC_URL"), "https://kart.openpak.org"), "/"),
		OpenPakURL:   strings.TrimRight(or(get("MKTV_OPENPAK_URL"), "https://openpak.org"), "/"),
		CoreURL:      strings.TrimRight(get("MKTV_CORE_URL"), "/"),
		SSOSecret:    get("SSO_CLIENT_OPENKART_SECRET"),
		SessionKey:   get("MKTV_SESSION_KEY"),
		MK8DURL:      strings.TrimRight(get("MKTV_MK8D_API_URL"), "/"),
		MK7URL:       strings.TrimRight(get("MKTV_MK7_API_URL"), "/"),
		MKWURL:       strings.TrimRight(get("MKTV_MKW_API_URL"), "/"),
		MKWKey:       get("MKTV_MKW_API_KEY"),
		CoursesFiles: map[string]string{},
		ContactEmail: or(get("MKTV_CONTACT_EMAIL"), "contact@openpak.org"),
	}
	if c.APIURL == "" {
		c.APIURL = strings.TrimRight(get("MKTV_MK8_API_URL"), "/")
	}
	for slug, env := range map[string]string{"mk8": "MKTV_COURSES_FILE", "mk8d": "MKTV_MK8D_COURSES_FILE", "mk7": "MKTV_MK7_COURSES_FILE", "mkw": "MKTV_MKW_COURSES_FILE"} {
		if v := get(env); v != "" {
			c.CoursesFiles[slug] = v
		}
	}
	if c.APIURL == "" && c.MK8DURL == "" && c.MK7URL == "" && c.MKWURL == "" {
		return c, fmt.Errorf("no game is configured: set at least one of MKTV_API_URL, MKTV_MK8D_API_URL, MKTV_MK7_API_URL, MKTV_MKW_API_URL")
	}
	for name, v := range map[string]string{"MKTV_API_URL": c.APIURL, "MKTV_MK8D_API_URL": c.MK8DURL, "MKTV_MK7_API_URL": c.MK7URL, "MKTV_MKW_API_URL": c.MKWURL, "MKTV_PUBLIC_URL": c.PublicURL, "MKTV_CORE_URL": c.CoreURL} {
		if v == "" {
			continue
		}
		if u, err := url.Parse(v); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return c, fmt.Errorf("%s %q is not an http(s) URL", name, v)
		}
	}
	if c.CoreURL != "" && c.SSOSecret != "" && len(c.SessionKey) < 32 {
		return c, fmt.Errorf("MKTV_SESSION_KEY must be at least 32 characters when sign-in is on")
	}
	return c, nil
}

func or(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
