package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jyablonski/arc/internal/ai"
)

// HealthChecks reports MCP state for "arc ai health". Everything here is
// offline and read-only: it compares canonical against each provider's file and
// checks that referenced credentials actually exist in the environment.
//
// These are warnings, not failures. A machine with no canonical MCP file at all
// is a normal state, not a broken one.
func HealthChecks(m *Manager) []ai.HealthCheck {
	if _, err := os.Stat(m.paths.CanonicalFile); err != nil {
		// Nothing to check, and nothing worth reporting: the user has simply
		// not adopted a canonical MCP store.
		return nil
	}

	var checks []ai.HealthCheck
	res, err := m.List()
	if err != nil {
		return []ai.HealthCheck{{
			Category: "mcp", Name: "mcp", Status: ai.HealthWarn,
			Detail: "cannot read MCP config: " + err.Error(),
		}}
	}

	checks = append(checks, driftCheck(res))
	if c, ok := envRefCheck(res); ok {
		checks = append(checks, c)
	}
	if c, ok := needsAuthCheck(m.paths.ClaudeAuthCache); ok {
		checks = append(checks, c)
	}
	return checks
}

// driftCheck summarizes how far the providers have wandered from canonical.
func driftCheck(res ListResult) ai.HealthCheck {
	c := ai.HealthCheck{Category: "mcp", Name: "mcp"}
	var drift, conflicts, missing, unsupported int
	for _, s := range res.Servers {
		for _, ps := range s.Providers {
			switch ps.Status {
			case StatusDrift:
				drift++
			case StatusConflict:
				conflicts++
			case StatusMissing:
				missing++
			case StatusUnsupported:
				unsupported++
			}
		}
	}

	var problems []string
	if missing > 0 {
		problems = append(problems, fmt.Sprintf("%d missing", missing))
	}
	if drift > 0 {
		problems = append(problems, fmt.Sprintf("%d drifted", drift))
	}
	if conflicts > 0 {
		problems = append(problems, fmt.Sprintf("%d conflicting", conflicts))
	}

	if len(problems) == 0 {
		detail := fmt.Sprintf("%d MCP configuration entries in sync across providers", len(res.Servers))
		if unsupported > 0 {
			detail += fmt.Sprintf(" (%d provider slot(s) unsupported by dialect)", unsupported)
		}
		c.Status = ai.HealthOK
		c.Detail = detail
		return c
	}
	c.Status = ai.HealthWarn
	c.Detail = fmt.Sprintf("%s provider slot(s)", strings.Join(problems, ", "))
	c.Hint = "run 'arc mcp sync' (see 'arc mcp list')"
	if conflicts > 0 && drift == 0 && missing == 0 {
		c.Hint = "run 'arc mcp list' to review, then 'arc mcp sync --force' to overwrite"
	}
	return c
}

// envRefCheck catches the failure that actually bites: an entry wired up
// correctly everywhere whose token variable is not exported, so it fails at
// connect time in every tool at once.
func envRefCheck(res ListResult) (ai.HealthCheck, bool) {
	missing := map[string][]string{}
	total := 0
	for _, s := range res.Servers {
		for _, ref := range s.EnvRefs {
			total++
			if _, ok := os.LookupEnv(ref); !ok {
				missing[ref] = append(missing[ref], s.Name)
			}
		}
	}
	if total == 0 {
		return ai.HealthCheck{}, false
	}

	c := ai.HealthCheck{Category: "mcp", Name: "mcp env"}
	if len(missing) == 0 {
		c.Status = ai.HealthOK
		c.Detail = fmt.Sprintf("%d referenced env var(s) set", total)
		return c, true
	}
	names := make([]string, 0, len(missing))
	for ref := range missing {
		names = append(names, "$"+ref)
	}
	sort.Strings(names)
	c.Status = ai.HealthWarn
	c.Detail = fmt.Sprintf("%s not set in this shell", strings.Join(names, ", "))
	c.Hint = "export the variable(s) in your shell profile; MCP entries referencing them will fail to authenticate"
	return c, true
}

// needsAuthCheck surfaces Claude Code's own record of remote MCP entries whose
// OAuth needs redoing. arc only reads this file.
func needsAuthCheck(path string) (ai.HealthCheck, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ai.HealthCheck{}, false
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) == 0 {
		return ai.HealthCheck{}, false
	}
	names := make([]string, 0, len(entries))
	for k := range entries {
		names = append(names, k)
	}
	sort.Strings(names)
	return ai.HealthCheck{
		Category: "mcp",
		Name:     "mcp auth",
		Status:   ai.HealthWarn,
		Detail:   fmt.Sprintf("claude recorded %d MCP entries needing auth: %s", len(names), strings.Join(names, ", ")),
		Hint:     "run '/mcp' in Claude Code to re-authenticate, or remove the entries if unused",
	}, true
}
