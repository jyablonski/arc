package cursor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
)

const apiBase = "https://cursor.com/api"

type Provider struct {
	HTTPClient *http.Client
	HomeDir    string
}

func (p *Provider) Name() string { return "cursor" }

func (p *Provider) Usage(ctx context.Context) (ai.UsageReport, error) {
	home := p.HomeDir
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return ai.UsageReport{}, fmt.Errorf("user home: %w", err)
		}
	}
	dbPath := StateDBPath(home)
	rawTok, err := ReadAccessTokenFromDB(dbPath)
	if err != nil {
		return ai.UsageReport{}, err
	}
	userID, jwt, err := splitSessionToken(rawTok)
	if err != nil {
		return ai.UsageReport{}, err
	}

	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	dash, planLabel, planOK, derr := fetchDashboardAndPlan(ctx, client, jwt)
	if derr != nil {
		rep, rerr := fetchRESTUsageReport(ctx, client, userID, jwt)
		if rerr != nil {
			return ai.UsageReport{}, fmt.Errorf("cursor api2: %v; fallback cursor.com/api: %w", derr, rerr)
		}
		return rep, nil
	}

	planLower := strings.ToLower(strings.TrimSpace(planLabel))
	if shouldUseEnterpriseTeamREST(dash, planLower) ||
		shouldUseRESTTeamNoLimit(dash, planLower) ||
		shouldUseUnknownREST(dash, planLower, !planOK) ||
		shouldUseRESTLimitMissingNoTotal(dash, planLower) {
		return fetchRESTUsageReport(ctx, client, userID, jwt)
	}

	return reportFromDashboard(dash, planLower, planLabel)
}

func fetchRESTUsageReport(ctx context.Context, client *http.Client, userID, jwt string) (ai.UsageReport, error) {
	body, err := cursorGET(ctx, client, "/usage?user="+userID, userID, jwt)
	if err != nil {
		return ai.UsageReport{}, err
	}
	rep, err := reportFromUsageBody(body)
	if err != nil {
		return ai.UsageReport{}, err
	}
	if len(rep.Windows) == 0 {
		if stripeBody, err := cursorGET(ctx, client, "/auth/stripe", userID, jwt); err == nil {
			var stripe map[string]any
			if json.Unmarshal(stripeBody, &stripe) == nil {
				if rep.Extra == nil {
					rep.Extra = map[string]any{}
				}
				rep.Extra["stripe"] = stripe
				rep.Windows = append(rep.Windows, ai.UsageWindow{
					Label:  "billing",
					Detail: "usage-based or non-request quota — see stripe in JSON / Cursor dashboard",
				})
			}
		}
	}
	sortWindows(rep.Windows)
	return rep, nil
}

func buildCookie(userID, jwt string) string {
	cookieValue := jwt
	if !strings.Contains(cookieValue, "::") && !strings.Contains(cookieValue, "%3A%3A") {
		cookieValue = userID + "%3A%3A" + jwt
	} else if strings.Contains(cookieValue, "::") && !strings.Contains(cookieValue, "%3A%3A") {
		cookieValue = strings.ReplaceAll(cookieValue, "::", "%3A%3A")
	}
	return "WorkosCursorSessionToken=" + cookieValue
}

func cursorGET(ctx context.Context, client *http.Client, path, userID, jwt string) ([]byte, error) {
	url := apiBase + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", buildCookie(userID, jwt))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://cursor.com")
	req.Header.Set("Referer", "https://cursor.com/dashboard")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cursor API %s: %s — %s", url, resp.Status, truncate(string(b), 300))
	}
	return b, nil
}

type modelUsage struct {
	NumRequests     float64  `json:"numRequests"`
	MaxRequestUsage *float64 `json:"maxRequestUsage"`
}

func reportFromUsageBody(body []byte) (ai.UsageReport, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return ai.UsageReport{}, fmt.Errorf("usage JSON: %w", err)
	}
	var windows []ai.UsageWindow
	extra := map[string]any{}

	for k, raw := range top {
		if k == "startOfMonth" {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				extra["start_of_month"] = s
			}
			continue
		}
		var probe map[string]any
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		if _, ok := probe["numRequests"]; !ok {
			continue
		}
		var m modelUsage
		_ = json.Unmarshal(raw, &m)
		label := fmt.Sprintf("%s requests", k)
		var pct float64
		if m.MaxRequestUsage != nil && *m.MaxRequestUsage > 0 {
			pct = 100.0 * m.NumRequests / *m.MaxRequestUsage
		}
		windows = append(windows, ai.UsageWindow{
			Label:       label,
			PercentUsed: pct,
			Detail:      fmt.Sprintf("%.0f / %.0f", m.NumRequests, derefMax(m.MaxRequestUsage)),
		})
	}

	return ai.UsageReport{Windows: windows, Extra: extra}, nil
}

func derefMax(m *float64) float64 {
	if m == nil {
		return 0
	}
	return *m
}

func sortWindows(w []ai.UsageWindow) {
	sort.Slice(w, func(i, j int) bool { return w[i].Label < w[j].Label })
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
