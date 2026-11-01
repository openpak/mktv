package site

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Sign-in goes through OpenPak's public API (openpak.org/api/v1): POST /token with the account's
// email and password mints a bearer, GET /me/wiiu names the Wii U identity behind the account,
// and the bearer is revoked straight away. The password is forwarded and forgotten; nothing
// here keeps a token.

var (
	errBadLogin  = errors.New("invalid credentials")
	errNoWiiU    = errors.New("no Wii U identity")
	errTwoFactor = errors.New("two-factor code required")
	errCoreDown  = errors.New("account service unavailable")
)

type twoFactorChallenge struct{ Challenge string }

func (e twoFactorChallenge) Error() string { return errTwoFactor.Error() }

type coreClient struct {
	base string
	http *http.Client
}

func newCoreClient(base string) *coreClient {
	return &coreClient{base: base, http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *coreClient) do(ctx context.Context, method, path, bearer string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, b, err
}

// login signs in with email and password. A two-factor account returns twoFactorChallenge.
func (c *coreClient) login(ctx context.Context, email, password string) (Player, error) {
	status, body, err := c.do(ctx, "POST", "/api/v1/token", "", map[string]string{"email": email, "password": password})
	return c.finish(ctx, status, body, err)
}

func (c *coreClient) login2FA(ctx context.Context, challenge, code string) (Player, error) {
	status, body, err := c.do(ctx, "POST", "/api/v1/token/2fa", "", map[string]string{"challenge": challenge, "code": code})
	return c.finish(ctx, status, body, err)
}

func (c *coreClient) finish(ctx context.Context, status int, body []byte, err error) (Player, error) {
	if err != nil {
		return Player{}, errCoreDown
	}
	var t struct {
		Token     string `json:"token"`
		TwoFactor bool   `json:"two_factor_required"`
		Challenge string `json:"challenge"`
	}
	_ = json.Unmarshal(body, &t)
	switch {
	case status == http.StatusAccepted && t.TwoFactor:
		return Player{}, twoFactorChallenge{t.Challenge}
	case status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusConflict || status == http.StatusGone:
		return Player{}, errBadLogin
	case status != http.StatusCreated && status != http.StatusOK || t.Token == "":
		return Player{}, errCoreDown
	}
	// Revoke on every path out, so a failure below does not leave a live token behind.
	defer c.do(context.WithoutCancel(ctx), "DELETE", "/api/v1/token", t.Token, nil)

	status, body, err = c.do(ctx, "GET", "/api/v1/me/wiiu", t.Token, nil)
	if err != nil {
		return Player{}, errCoreDown
	}
	if status == http.StatusNotFound {
		return Player{}, errNoWiiU
	}
	if status != http.StatusOK {
		return Player{}, errCoreDown
	}
	var id struct {
		PID      json.Number `json:"pid"`
		Username string      `json:"username"`
		MiiName  string      `json:"mii_name"`
	}
	if json.Unmarshal(body, &id) != nil {
		return Player{}, errCoreDown
	}
	if n, err := strconv.ParseInt(id.PID.String(), 10, 64); err != nil || n <= 0 {
		return Player{}, errNoWiiU
	}
	return Player{PID: id.PID.String(), Username: id.Username, MiiName: id.MiiName}, nil
}
