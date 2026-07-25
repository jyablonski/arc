package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jyablonski/arc/internal/mcp"
	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var (
	mcpDryRun       bool
	mcpSyncForce    bool
	mcpAddForce     bool
	mcpProvider     string
	mcpAddType      string
	mcpAddCommand   string
	mcpAddArgs      []string
	mcpAddEnv       []string
	mcpAddURL       string
	mcpAddHeaders   []string
	mcpAddDisabled  bool
	mcpAddProviders []string
)

var ErrMCPConflict = errors.New("mcp: unresolved conflicts")

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage shared MCP servers across AI providers",
	Long: `Manage MCP servers across Claude, Codex, Cursor, and opencode.

The canonical store is ~/ai/mcp.json, alongside the shared skills and AGENTS.md.
Unlike skills and rules, MCP config cannot be symlinked: three of the four
providers keep their servers inside a larger file the tool also owns. arc
renders canonical into each provider's dialect and merges it in, rewriting only
the servers it wrote itself and leaving hand-configured ones alone.

Credentials belong in the environment: reference them as {env:VAR_NAME} and arc
translates that into each provider's mechanism. Literal tokens are rejected.`,
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List canonical MCP servers and their per-provider status",
	Long: `Shows every canonical server with one column per provider.

Statuses: ok (matches), missing (not written yet), drift (arc owns it and it
was edited elsewhere), conflict (configured by hand and differs — sync will not
touch it), unsupported (the provider's dialect cannot express it), disabled,
excluded (restricted to other providers).`,
	RunE: runMCPList,
}

var mcpSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Render canonical MCP servers into every provider",
	Long: `Merges the canonical server list into each provider's config file.

Sync is one-way: ~/ai/mcp.json is the source of truth. Servers arc previously
wrote are updated or removed to match; servers it did not write are reported as
conflicts and left untouched unless --force is passed.

Exits non-zero if any conflict is unresolved.`,
	RunE: runMCPSync,
}

var mcpImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Seed canonical from servers already configured in the providers",
	Long: `Reads every provider's existing MCP servers and adds the ones canonical does
not have yet. This is the migration path onto a shared store.

Import never overwrites canonical: a name that already exists is skipped when
identical and reported as a conflict when it differs. A server carrying a
literal credential is rejected rather than copied into a shared file.

Nothing is pushed back out; run 'arc mcp sync' afterwards.`,
	RunE: runMCPImport,
}

var mcpAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a server to canonical and sync it into every provider",
	Long: `Adds one MCP server to ~/ai/mcp.json and immediately syncs it out.

Examples:

  arc mcp add ctx7 --command uvx --arg context7-mcp@latest
  arc mcp add homelab --url http://mcp.home/mcp \
    --header 'Authorization=Bearer {env:HOMELAB_MCP_TOKEN}'

Use --restrict-to to restrict a server to a subset of tools.`,
	RunE: runMCPAdd,
}

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a server from canonical and sweep it from the providers",
	Long: `Deletes a server from ~/ai/mcp.json, then syncs. Provider copies that arc
wrote are removed; hand-configured copies of the same name are left alone.`,
	RunE: runMCPRemove,
}

var mcpValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check canonical MCP servers against the schema and each dialect",
	Long: `Validates every canonical server: schema correctness and inline-credential
checks (fatal), plus per-provider translation warnings and referenced
environment variables that are not set.

Exits non-zero only on fatal issues; a server that merely cannot be expressed in
one provider's dialect is a warning.`,
	RunE: runMCPValidate,
}

// newMCPManager builds the manager every subcommand uses. It always honors
// --provider: add and remove sync as their last step, so a filter that applied
// only to `sync` would silently write to providers the user excluded.
func newMCPManager(forceConflicts bool) (*mcp.Manager, error) {
	paths := mcp.DefaultPaths()
	providers := mcp.DefaultProviders(paths)
	if strings.TrimSpace(mcpProvider) != "" {
		names, err := parseProviderFilter(mcpProvider, func(f []string) error { return nil })
		if err != nil {
			return nil, err
		}
		providers, err = mcp.FilterProviders(providers, names)
		if err != nil {
			return nil, err
		}
	}
	return mcp.New(mcp.Config{
		Paths:     paths,
		Providers: providers,
		DryRun:    mcpDryRun,
		Force:     forceConflicts,
	}), nil
}

func runMCPList(cmd *cobra.Command, args []string) error {
	_ = args
	m, err := newMCPManager(false)
	if err != nil {
		return err
	}
	res, err := m.List()
	if err != nil {
		return err
	}
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		return encodeMCPJSON(res)
	}
	mcp.PrintListHuman(os.Stdout, m.Providers(), res)
	return nil
}

func runMCPSync(cmd *cobra.Command, args []string) error {
	_ = args
	m, err := newMCPManager(mcpSyncForce)
	if err != nil {
		return err
	}
	res, err := m.Sync()
	if err != nil {
		return err
	}
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		if err := encodeMCPJSON(res); err != nil {
			return err
		}
	} else {
		output.Header("MCP sync")
		mcp.PrintSyncHuman(os.Stdout, res)
	}
	if res.Failures() > 0 {
		return fmt.Errorf("%d provider(s) failed to sync", res.Failures())
	}
	if res.Conflicts() > 0 {
		return ErrMCPConflict
	}
	return nil
}

func runMCPImport(cmd *cobra.Command, args []string) error {
	_ = args
	m, err := newMCPManager(false)
	if err != nil {
		return err
	}
	res, err := m.Import()
	if err != nil {
		return err
	}
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		return encodeMCPJSON(res)
	}
	output.Header("MCP import")
	mcp.PrintImportHuman(os.Stdout, res)
	return nil
}

func runMCPAdd(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one server name is required")
	}
	server, err := buildServerFromFlags()
	if err != nil {
		return err
	}
	m, err := newMCPManager(false)
	if err != nil {
		return err
	}
	res, err := m.Add(args[0], server, mcpAddForce)
	if err != nil {
		return err
	}
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		return encodeMCPJSON(res)
	}
	mcp.PrintSyncHuman(os.Stdout, res)
	if res.Conflicts() > 0 {
		return ErrMCPConflict
	}
	return nil
}

func runMCPRemove(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one server name is required")
	}
	m, err := newMCPManager(false)
	if err != nil {
		return err
	}
	res, err := m.Remove(args[0])
	if err != nil {
		return err
	}
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		return encodeMCPJSON(res)
	}
	mcp.PrintSyncHuman(os.Stdout, res)
	return nil
}

func runMCPValidate(cmd *cobra.Command, args []string) error {
	_ = args
	m, err := newMCPManager(false)
	if err != nil {
		return err
	}
	issues, err := m.Validate()
	if err != nil {
		return err
	}
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		if err := encodeMCPJSON(issues); err != nil {
			return err
		}
	} else if len(issues) == 0 {
		output.Success("all MCP servers valid")
	} else {
		for _, issue := range issues {
			label := issue.Server
			if issue.Provider != "" {
				label = fmt.Sprintf("%s (%s)", issue.Server, issue.Provider)
			}
			msg := fmt.Sprintf("%s: %s", label, issue.Error)
			if issue.Fatal {
				output.Error(msg)
			} else {
				output.Warning(msg)
			}
		}
	}

	var fatal int
	for _, issue := range issues {
		if issue.Fatal {
			fatal++
		}
	}
	if fatal > 0 {
		return fmt.Errorf("%d invalid MCP server(s)", fatal)
	}
	return nil
}

// buildServerFromFlags turns the add flags into a canonical server, inferring
// the transport from whichever of --command/--url was given.
func buildServerFromFlags() (mcp.Server, error) {
	var s mcp.Server
	hasCommand := strings.TrimSpace(mcpAddCommand) != ""
	hasURL := strings.TrimSpace(mcpAddURL) != ""
	if hasCommand == hasURL {
		return s, fmt.Errorf("exactly one of --command or --url is required")
	}

	switch strings.ToLower(strings.TrimSpace(mcpAddType)) {
	case "":
		if hasURL {
			s.Type = mcp.TypeHTTP
		} else {
			s.Type = mcp.TypeStdio
		}
	case "stdio":
		s.Type = mcp.TypeStdio
	case "http":
		s.Type = mcp.TypeHTTP
	case "sse":
		s.Type = mcp.TypeSSE
	default:
		return s, fmt.Errorf("--type must be stdio, http, or sse")
	}

	s.Command = strings.TrimSpace(mcpAddCommand)
	s.Args = mcpAddArgs
	s.URL = strings.TrimSpace(mcpAddURL)

	env, err := parseKeyValueFlags(mcpAddEnv, "--env")
	if err != nil {
		return s, err
	}
	s.Env = env

	headers, err := parseKeyValueFlags(mcpAddHeaders, "--header")
	if err != nil {
		return s, err
	}
	s.Headers = headers

	if mcpAddDisabled {
		disabled := false
		s.Enabled = &disabled
	}
	s.Providers = mcpAddProviders
	return s, nil
}

func parseKeyValueFlags(pairs []string, flag string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s expects KEY=VALUE, got %q", flag, pair)
		}
		out[strings.TrimSpace(key)] = value
	}
	return out, nil
}

func encodeMCPJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpSyncCmd)
	mcpCmd.AddCommand(mcpImportCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpRemoveCmd)
	mcpCmd.AddCommand(mcpValidateCmd)

	mcpCmd.PersistentFlags().BoolVar(&mcpDryRun, "dry-run", false, "Print planned actions without modifying any file")
	mcpCmd.PersistentFlags().StringVar(&mcpProvider, "provider", "", "Restrict to a comma-separated provider list (claude, codex, cursor, opencode)")

	mcpSyncCmd.Flags().BoolVar(&mcpSyncForce, "force", false, "Overwrite provider entries arc did not write")
	mcpAddCmd.Flags().BoolVar(&mcpAddForce, "force", false, "Replace an existing canonical server")

	mcpAddCmd.Flags().StringVar(&mcpAddType, "type", "", "Transport: stdio, http, or sse (inferred when omitted)")
	mcpAddCmd.Flags().StringVar(&mcpAddCommand, "command", "", "Executable for a stdio server")
	mcpAddCmd.Flags().StringArrayVar(&mcpAddArgs, "arg", nil, "Argument for a stdio server (repeatable)")
	mcpAddCmd.Flags().StringArrayVar(&mcpAddEnv, "env", nil, "KEY=VALUE for a stdio server (repeatable)")
	mcpAddCmd.Flags().StringVar(&mcpAddURL, "url", "", "Endpoint for an http or sse server")
	mcpAddCmd.Flags().StringArrayVar(&mcpAddHeaders, "header", nil, "KEY=VALUE header (repeatable); use {env:VAR} for credentials")
	mcpAddCmd.Flags().BoolVar(&mcpAddDisabled, "disabled", false, "Keep the server in canonical but disabled")
	mcpAddCmd.Flags().StringArrayVar(&mcpAddProviders, "restrict-to", nil, "Restrict this server to the named provider (repeatable)")
}
