package cursor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var userIDRE = regexp.MustCompile(`user_[A-Za-z0-9]+`)

func splitSessionToken(access string) (userID string, jwt string, err error) {
	access = strings.TrimSpace(access)
	if access == "" {
		return "", "", fmt.Errorf("empty access token")
	}
	if strings.Contains(access, "::") {
		parts := strings.SplitN(access, "::", 2)
		if len(parts) == 2 {
			uid := strings.TrimSpace(parts[0])
			jt := strings.TrimSpace(parts[1])
			if uid != "" && jt != "" {
				return uid, jt, nil
			}
		}
	}
	if strings.Contains(access, "%3A%3A") {
		dec, err := url.QueryUnescape(access)
		if err == nil && strings.Contains(dec, "::") {
			return splitSessionToken(dec)
		}
	}
	uid, err := userIDFromJWT(access)
	if err != nil {
		return "", "", err
	}
	return uid, access, nil
}

func userIDFromJWT(jwt string) (string, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("jwt: expected 3 segments")
	}
	payload := parts[1]
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(padBase64(payload))
		if err != nil {
			return "", fmt.Errorf("jwt payload base64: %w", err)
		}
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", fmt.Errorf("jwt claims: %w", err)
	}
	if m := userIDRE.FindString(claims.Sub); m != "" {
		return m, nil
	}
	if m := userIDRE.FindString(jwt); m != "" {
		return m, nil
	}
	return "", fmt.Errorf("jwt: no user_* id in sub")
}

func padBase64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}
