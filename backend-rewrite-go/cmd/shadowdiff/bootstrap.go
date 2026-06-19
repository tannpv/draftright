package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func parseAccessToken(body []byte) (string, error) {
	var r struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("no access_token in response: %s", body)
	}
	return r.AccessToken, nil
}

// parseRefreshToken extracts the refresh_token from a login response. The
// refresh token is a stateless JWT signed with JWT_REFRESH_SECRET (no DB row),
// so a token minted at bootstrap survives the per-fixture DB reset and verifies
// on BOTH backends. Exposed as {{user_refresh_token}} so a /auth/refresh fixture
// has a valid body without chaining requests.
func parseRefreshToken(body []byte) (string, error) {
	var r struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.RefreshToken == "" {
		return "", fmt.Errorf("no refresh_token in response: %s", body)
	}
	return r.RefreshToken, nil
}

func parseExtToken(body []byte) (string, error) {
	var r struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.Token == "" {
		return "", fmt.Errorf("no token in ext-token response: %s", body)
	}
	return r.Token, nil
}

// shadowDeviceID is a fixed v4 UUID used as the device_id when minting the
// shadow-harness extension token. The Go handler's ValidateMint requires a
// UUID v4 (class-validator @IsUUID('4') parity); without this field the
// mint call returns 400 "device_id must be a UUID".
const shadowDeviceID = "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5"

// bootstrapTokens logs in as the seeded user + admin against `base` and mints an
// extension token, returning the substitution map. Run once per gate, before
// fixtures. Same JWT secret on both backends means these verify on each.
func bootstrapTokens(c *http.Client, base, userEmail, userPass, adminEmail, adminPass string) (map[string]string, error) {
	post := func(path string, payload any, hdr map[string]string) ([]byte, error) {
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s -> %d: %s", path, resp.StatusCode, body)
		}
		return body, nil
	}

	vars := map[string]string{}
	ub, err := post("/auth/login", map[string]string{"email": userEmail, "password": userPass}, nil)
	if err != nil {
		return nil, err
	}
	if vars["user_token"], err = parseAccessToken(ub); err != nil {
		return nil, err
	}
	if vars["user_refresh_token"], err = parseRefreshToken(ub); err != nil {
		return nil, err
	}
	ab, err := post("/admin/auth/login", map[string]string{"email": adminEmail, "password": adminPass}, nil)
	if err != nil {
		return nil, err
	}
	if vars["admin_token"], err = parseAccessToken(ab); err != nil {
		return nil, err
	}
	eb, err := post("/auth/extension-tokens",
		map[string]string{"device_id": shadowDeviceID, "device_name": "shadow"},
		map[string]string{"Authorization": "Bearer " + vars["user_token"]})
	if err != nil {
		return nil, err
	}
	if vars["ext_token"], err = parseExtToken(eb); err != nil {
		return nil, err
	}
	return vars, nil
}
