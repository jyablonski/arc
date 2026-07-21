package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type HealthStatus string

const (
	HealthOK   HealthStatus = "ok"
	HealthWarn HealthStatus = "warn"
	HealthFail HealthStatus = "fail"
)

// HealthCheck is one offline diagnostic about the AI toolchain: whether a
// provider is authenticated, a CLI is installed, or a local config is sane.
type HealthCheck struct {
	Category string       `json:"category"`
	Name     string       `json:"name"`
	Status   HealthStatus `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Hint     string       `json:"hint,omitempty"`
}

type HealthReport struct {
	FetchedAt time.Time     `json:"fetched_at"`
	Checks    []HealthCheck `json:"checks"`
}

func (r HealthReport) HasFailure() bool {
	for _, c := range r.Checks {
		if c.Status == HealthFail {
			return true
		}
	}
	return false
}

//go:generate go tool moq -rm -out healthchecker_moq.go . HealthChecker

// HealthChecker reports the offline health of one AI tool. Implementations read
// local credential/config state only and never touch the network.
type HealthChecker interface {
	Name() string
	Health(ctx context.Context) []HealthCheck
}

// RunHealthCheckers gathers checks from the selected providers, in order.
func RunHealthCheckers(ctx context.Context, checkers []HealthChecker, filters []string) []HealthCheck {
	filter := normalizeFilter(filters)
	var checks []HealthCheck
	for _, c := range checkers {
		if len(filter) == 0 || filter[strings.ToLower(strings.TrimSpace(c.Name()))] {
			checks = append(checks, c.Health(ctx)...)
		}
	}
	return checks
}

// ToolCheck reports whether a CLI binary is resolvable on PATH.
func ToolCheck(bin, label string) HealthCheck {
	c := HealthCheck{Category: "tooling", Name: bin}
	if _, err := exec.LookPath(bin); err != nil {
		c.Status = HealthWarn
		c.Detail = label + " not found on PATH"
		return c
	}
	c.Status = HealthOK
	c.Detail = label + " installed"
	return c
}

// JWTExpiry extracts the `exp` claim from a JWT without verifying its signature.
// ok is false when the token is not a decodable three-part JWT or carries no exp.
func JWTExpiry(token string) (time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(padBase64(parts[1]))
		if err != nil {
			return time.Time{}, false
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
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

// TokenExpiryCheck builds an auth HealthCheck from a decoded expiry. hasRefresh
// softens an expired token to a warning, since the tool refreshes it on next use.
func TokenExpiryCheck(name string, exp time.Time, ok, hasRefresh bool, source, hint string) HealthCheck {
	c := HealthCheck{Category: "auth", Name: name, Hint: hint}
	src := ""
	if source != "" {
		src = " (" + source + ")"
	}
	switch {
	case !ok:
		c.Status = HealthOK
		c.Detail = "logged in; token expiry unknown" + src
	case time.Now().Before(exp):
		c.Status = HealthOK
		c.Detail = "token valid for " + compactDuration(time.Until(exp)) + src
	case hasRefresh:
		c.Status = HealthWarn
		c.Detail = "token expired " + compactDuration(time.Since(exp)) + " ago; refreshes on next use" + src
	default:
		c.Status = HealthFail
		c.Detail = "token expired " + compactDuration(time.Since(exp)) + " ago; sign in again" + src
	}
	return c
}

func compactDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Hour:
		return strconv.Itoa(max(1, int(d.Minutes()))) + "m"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	default:
		return strconv.Itoa(int(d.Hours())/24) + "d"
	}
}

func EncodeHealthJSON(w io.Writer, report HealthReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
