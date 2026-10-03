package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/hardware"
	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/setupdeps"
	"github.com/jyablonski/arc/internal/stats"
	"github.com/jyablonski/arc/internal/sysupdate"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// requireOutputGrammar asserts the frame every human-readable command shares:
// a self-identifying title, a rule as wide as the title line, one blank line,
// then a body that ends in exactly one closing line. It returns the lines.
func requireOutputGrammar(t *testing.T, out, title string) []string {
	t.Helper()
	require.True(t, strings.HasSuffix(out, "\n"), "output must end with a newline:\n%s", out)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), 4, "expected title, rule, blank and a closing line:\n%s", out)

	require.True(t, lines[0] == title || strings.HasPrefix(lines[0], title+" "), "title line %q should lead with %q", lines[0], title)
	ruleWidth := utf8.RuneCountInString(lines[1])
	require.Equal(t, strings.Repeat("─", ruleWidth), lines[1], "second line must be the rule")
	require.GreaterOrEqual(t, ruleWidth, output.FrameWidth)
	require.LessOrEqual(t, ruleWidth, output.MaxFrameWidth)
	if lines[0] != title {
		require.Equal(t, ruleWidth, utf8.RuneCountInString(lines[0]), "meta must end at the rule's right edge")
	}
	require.Empty(t, lines[2], "a blank line separates the rule from the body")

	last := len(lines) - 1
	require.NotEmpty(t, lines[last], "output must end with a closing line")
	if last > 3 {
		require.Empty(t, lines[last-1], "the closing line is set off from the body by a blank line")
	}
	for i, line := range lines {
		require.LessOrEqual(t, utf8.RuneCountInString(line), output.MaxFrameWidth, "line %d overflows the frame: %q", i, line)
		require.NotContains(t, line, "===", "legacy banner on line %d", i)
		require.NotContains(t, line, "\x1b[", "piped output must not carry color")
		if i > 0 && line == "" {
			require.NotEmpty(t, lines[i-1], "two consecutive blank lines at line %d:\n%s", i, out)
		}
	}
	return lines
}

func grammarEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("COLUMNS", "")
	resetJSONFlag(t)
}

func okRunner(runOut string, runErr error) *boundary.ShellRunnerMock {
	return &boundary.ShellRunnerMock{
		CommandExistsFunc:  func(string) bool { return true },
		RunFunc:            func(string, ...string) (string, error) { return runOut, runErr },
		RunInteractiveFunc: func(string, ...string) error { return nil },
	}
}

func TestOutputGrammar_everyCommandSharesOneFrame(t *testing.T) {
	tests := []struct {
		name  string
		title string
		// setup prepares state and returns the command to run with its args.
		setup   func(t *testing.T) (*cobra.Command, []string)
		wantErr bool
		// closing is a substring of the final line.
		closing string
		// contains are substrings expected somewhere in the body.
		contains []string
	}{
		{
			name:  "stats",
			title: "arc stats",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				t.Setenv("XDG_STATE_HOME", t.TempDir())
				ts := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
				require.NoError(t, stats.Append(stats.Entry{Timestamp: ts, Command: "clean", OK: true, DurationMS: 100}))
				return statsCmd, nil
			},
			closing:  "1 command · 1 invocation · 0 failures",
			contains: []string{"command  count  failures  last used"},
		},
		{
			name:  "stats with nothing recorded",
			title: "arc stats",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				t.Setenv("XDG_STATE_HOME", t.TempDir())
				return statsCmd, nil
			},
			closing: "· no invocations recorded yet",
		},
		{
			name:  "validate",
			title: "arc validate",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				t.Cleanup(setAppForTest(newApp(platform.Linux)))
				setRunner(t, okRunner("", nil))
				return validateCmd, nil
			},
			closing:  "✓ all required tools available",
			contains: []string{"✓  pacman", "required"},
		},
		{
			name:  "validate with a missing required tool",
			title: "arc validate",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				t.Cleanup(setAppForTest(newApp(platform.Linux)))
				setRunner(t, &boundary.ShellRunnerMock{CommandExistsFunc: func(name string) bool { return name != "git" }})
				return validateCmd, nil
			},
			wantErr:  true,
			closing:  "✗ 1 required tool missing · arc setup",
			contains: []string{"✗  git"},
		},
		{
			name:  "docker clean",
			title: "arc docker clean",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				setRunner(t, okRunner("Deleted Images:\nsha256:abc\n\nTotal reclaimed space: 1.2GB\n", nil))
				return dockerCmd, nil
			},
			closing:  "✓ docker cleanup complete",
			contains: []string{"✓ images pruned (1.2GB)", "✓ volumes pruned (1.2GB)"},
		},
		{
			name:  "docker clean with failed prunes",
			title: "arc docker clean",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				setRunner(t, okRunner("", errors.New("daemon not running")))
				return dockerCmd, nil
			},
			closing:  "⚠ 3 of 3 prunes failed",
			contains: []string{"⚠ images not pruned: daemon not running"},
		},
		{
			name:  "clean",
			title: "arc clean",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				mgr := &pkgmgr.ManagerMock{CleanFunc: func(pkgmgr.CleanOptions) error {
					output.Success("package cache cleaned")
					return nil
				}}
				t.Cleanup(setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr}))
				old := cleanUpdateLogs
				cleanUpdateLogs = func() (sysupdate.LogCleanupResult, error) {
					return sysupdate.LogCleanupResult{Files: 2, Bytes: 2048}, nil
				}
				t.Cleanup(func() { cleanUpdateLogs = old })
				resetCleanFlags()
				return cleanCmd, nil
			},
			closing:  "✓ clean complete",
			contains: []string{"✓ package cache cleaned", "✓ removed 2 update logs (2.0 KiB)"},
		},
		{
			name:  "setup",
			title: "arc setup",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				// An installer that reports nothing: the verdict sits directly
				// under the title instead of after a stray blank line.
				installer := &setupdeps.InstallerMock{InstallFunc: func() error { return nil }}
				t.Cleanup(setAppForTest(&App{Platform: platform.Darwin, Setup: installer}))
				return setupCmd, nil
			},
			closing: "✓ setup complete · arc validate",
		},
		{
			name:  "parts",
			title: "arc parts",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				reporter := &hardware.ReporterMock{ShowFunc: func(components []string) error {
					for _, c := range components {
						output.Section(c)
						fmt.Println("vendor: example")
					}
					return nil
				}}
				t.Cleanup(setAppForTest(&App{Platform: platform.Linux, Hardware: reporter}))
				partsComponent = ""
				return partsCmd, nil
			},
			closing:  "· 5 components",
			contains: []string{"mobo\nvendor: example\n\ncpu\n"},
		},
		{
			name:  "update uv",
			title: "arc update uv",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				setRunner(t, okRunner("", nil))
				return updateUvCmd, nil
			},
			closing: "✓ uv updated",
		},
		{
			name:  "incident",
			title: "arc incident",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
				t.Cleanup(srv.Close)
				t.Setenv("SLACK_WEBHOOK_URL", srv.URL)
				incidentService, incidentSeverity, incidentDiscord = "api", "p1", false
				t.Cleanup(func() { incidentService, incidentSeverity = "unknown", "p3" })
				return incidentCmd, []string{"database is down"}
			},
			closing:  "✓ alert sent to 1 of 1 channel",
			contains: []string{"p1 · api", "· database is down"},
		},
		{
			name:  "incident with every channel failing",
			title: "arc incident",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
				t.Cleanup(srv.Close)
				t.Setenv("SLACK_WEBHOOK_URL", srv.URL)
				incidentService, incidentSeverity, incidentDiscord = "api", "p1", false
				t.Cleanup(func() { incidentService, incidentSeverity = "unknown", "p3" })
				return incidentCmd, []string{"database is down"}
			},
			wantErr: true,
			closing: "✗ alert sent to 0 of 1 channel",
		},
		{
			name:  "rules status",
			title: "arc rules status",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				writeFile(t, filepath.Join(root, "ai", "AGENTS.md"), "x\n")
				return rulesStatusCmd, nil
			},
			closing:  "out of sync · arc rules sync",
			contains: []string{"provider  status"},
		},
		{
			name:  "rules sync",
			title: "arc rules sync",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				writeFile(t, filepath.Join(root, "ai", "AGENTS.md"), "x\n")
				resetRulesFlags()
				return rulesSyncCmd, nil
			},
			closing:  "✓ rules synced",
			contains: []string{"· create symlink"},
		},
		{
			name:  "rules sync dry run",
			title: "arc rules sync",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				writeFile(t, filepath.Join(root, "ai", "AGENTS.md"), "x\n")
				rulesDryRun = true
				t.Cleanup(resetRulesFlags)
				return rulesSyncCmd, nil
			},
			closing:  "✓ dry run complete",
			contains: []string{"dry run\n", "· would create symlink"},
		},
		{
			name:  "skills sync",
			title: "arc skills sync",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"), "---\nname: foo\ndescription: d\n---\nbody\n")
				resetSkillsFlags()
				return skillsSyncCmd, nil
			},
			closing:  "linked",
			contains: []string{"· create symlink"},
		},
		{
			name:  "skills sync dry run",
			title: "arc skills sync",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"), "---\nname: foo\ndescription: d\n---\nbody\n")
				resetSkillsFlags()
				skillsDryRun = true
				t.Cleanup(resetSkillsFlags)
				return skillsSyncCmd, nil
			},
			// A dry run never reports its plan in the past tense.
			closing:  "to link",
			contains: []string{"· would create symlink"},
		},
		{
			name:  "skills sync with nothing to do",
			title: "arc skills sync",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				require.NoError(t, os.MkdirAll(filepath.Join(root, "ai", "skills"), 0o755))
				resetSkillsFlags()
				return skillsSyncCmd, nil
			},
			closing: "✓ already in sync",
		},
		{
			name:  "skills remove",
			title: "arc skills remove",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"), "---\nname: foo\ndescription: d\n---\nbody\n")
				resetSkillsFlags()
				return skillsRemoveCmd, []string{"foo"}
			},
			closing: "✓ removed foo",
		},
		{
			name:  "skills validate",
			title: "arc skills validate",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				require.NoError(t, os.MkdirAll(filepath.Join(root, "ai", "skills"), 0o755))
				resetSkillsFlags()
				return skillsValidateCmd, nil
			},
			closing: "✓ all skills valid",
		},
		{
			name:  "skills prune",
			title: "arc skills prune",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupSkillsEnv(t)
				require.NoError(t, os.MkdirAll(filepath.Join(root, "ai", "skills"), 0o755))
				resetSkillsFlags()
				return skillsPruneCmd, nil
			},
			closing: "· no dangling symlinks",
		},
		{
			name:  "mcp add",
			title: "arc mcp add",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				setupMCPEnv(t)
				resetMCPFlags()
				t.Cleanup(resetMCPFlags)
				mcpAddCommand = "uvx"
				mcpAddArgs = []string{"context7-mcp"}
				return mcpAddCmd, []string{"ctx7"}
			},
			closing:  "written",
			contains: []string{"provider  written  removed  conflicts  unsupported  path", "· add claude/ctx7\n"},
		},
		{
			name:  "mcp sync dry run",
			title: "arc mcp sync",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupMCPEnv(t)
				resetMCPFlags()
				t.Cleanup(resetMCPFlags)
				writeFile(t, filepath.Join(root, "ai", "mcp.json"), `{"mcpServers":{"ctx7":{"type":"stdio","command":"uvx"}}}`)
				mcpDryRun = true
				return mcpSyncCmd, nil
			},
			closing:  "to write",
			contains: []string{"dry run\n", "provider  to write  to remove", "· would add claude/ctx7\n"},
		},
		{
			name:  "mcp validate",
			title: "arc mcp validate",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				root := setupMCPEnv(t)
				resetMCPFlags()
				writeFile(t, filepath.Join(root, "ai", "mcp.json"), `{"mcpServers":{"ctx7":{"type":"stdio","command":"uvx"}}}`)
				return mcpValidateCmd, nil
			},
			closing: "✓ all MCP configuration entries valid",
		},
		{
			name:  "mcp import with nothing configured",
			title: "arc mcp import",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				setupMCPEnv(t)
				resetMCPFlags()
				return mcpImportCmd, nil
			},
			closing: "· nothing new to import into",
		},
		{
			name:  "ai health",
			title: "arc ai health",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				resetAIUsageCLIState(t)
				writeCodexSessionFixture(t)
				setupMCPEnv(t)
				aiHealthProvider = "claude"
				t.Cleanup(func() { aiHealthProvider = "" })
				return aiHealthCmd, nil
			},
			// No Claude credentials in the fixture home: the screen still
			// renders in full and the command reports the failure.
			wantErr:  true,
			closing:  "failed",
			contains: []string{"provider  check"},
		},
		{
			name:  "ai tokens",
			title: "arc ai tokens",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				resetAIUsageCLIState(t)
				writeCodexSessionFixture(t)
				return aiTokensCmd, nil
			},
			closing:  "api equiv",
			contains: []string{"codex/gpt-5-codex"},
		},
		{
			name:  "ai sessions",
			title: "arc ai sessions",
			setup: func(t *testing.T) (*cobra.Command, []string) {
				resetAIUsageCLIState(t)
				writeCodexSessionFixture(t)
				return aiSessionsCmd, nil
			},
			closing:  "1 session · 2.0M tokens · $11.25 api equiv",
			contains: []string{"tokens  api equiv  project", "$11.25"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grammarEnv(t)
			c, args := tt.setup(t)
			c.SetContext(context.Background())
			var err error
			out := captureStdout(t, func() { err = c.RunE(c, args) })
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			lines := requireOutputGrammar(t, out, tt.title)
			require.Contains(t, lines[len(lines)-1], tt.closing)
			for _, want := range tt.contains {
				require.Contains(t, out, want)
			}
		})
	}
}

// writeCodexSessionFixture points HOME at a temp dir holding one Codex
// session: 1M input and 1M output tokens on gpt-5-codex ($11.25 at defaults).
func writeCodexSessionFixture(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	writeFile(t, filepath.Join(home, ".codex", "sessions", "rollout.jsonl"),
		`{"type":"turn_context","timestamp":"2026-06-01T12:00:00Z","payload":{"model":"gpt-5-codex"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-06-01T12:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"output_tokens":1000000}}}}`+"\n")
}

// A command that fails while running reports the error once, in the shared
// glyph set, without cobra's usage text; a command that could not be parsed
// still gets its usage.
func TestRootCmd_usageOnlyForUsageErrors(t *testing.T) {
	grammarEnv(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(setAppForTest(newApp(platform.Linux)))
	setRunner(t, &boundary.ShellRunnerMock{CommandExistsFunc: func(string) bool { return false }})

	execute := func(args ...string) (string, error) {
		var cobraOut bytes.Buffer
		rootCmd.SetOut(&cobraOut)
		rootCmd.SetErr(&cobraOut)
		rootCmd.SetArgs(args)
		t.Cleanup(func() {
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
			rootCmd.SetArgs(nil)
		})
		var err error
		captureStdout(t, func() { err = rootCmd.Execute() })
		return cobraOut.String(), err
	}

	// Usage error first: running a command marks it as past parsing.
	validateCmd.SilenceUsage = false
	printed, err := execute("validate", "--no-such-flag")
	require.Error(t, err)
	require.Contains(t, printed, "Usage:")
	require.NotContains(t, printed, "Error:")

	printed, err = execute("validate")
	require.Error(t, err)
	require.Empty(t, printed, "a runtime failure must not print cobra's error or usage")
}
