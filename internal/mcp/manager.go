package mcp

import (
	"fmt"
	"os"
	"sort"

	"github.com/jyablonski/arc/internal/output"
)

type Status string

const (
	// StatusOK: present in the provider and matching canonical.
	StatusOK Status = "ok"
	// StatusMissing: belongs in the provider but is not there yet.
	StatusMissing Status = "missing"
	// StatusDrift: present, differs from canonical, and arc owns it — sync
	// will overwrite it.
	StatusDrift Status = "drift"
	// StatusConflict: present, differs, and arc did not put it there — sync
	// leaves it alone unless forced.
	StatusConflict Status = "conflict"
	// StatusUnsupported: the provider's dialect cannot express this server.
	StatusUnsupported Status = "unsupported"
	// StatusDisabled: disabled in canonical and correctly absent here.
	StatusDisabled Status = "disabled"
	// StatusExcluded: canonical restricts this server to other providers.
	StatusExcluded Status = "excluded"
)

type ProviderStatus struct {
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ServerEntry struct {
	Name      string                    `json:"name"`
	Type      ServerType                `json:"type"`
	Enabled   bool                      `json:"enabled"`
	EnvRefs   []string                  `json:"env_refs,omitempty"`
	Providers map[string]ProviderStatus `json:"providers"`
}

// UnmanagedEntry is a server configured in a provider that arc knows nothing
// about. Reporting these is how `list` stays honest about not being the whole
// picture.
type UnmanagedEntry struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

type ListResult struct {
	CanonicalFile string           `json:"canonical_file"`
	Servers       []ServerEntry    `json:"servers"`
	Unmanaged     []UnmanagedEntry `json:"unmanaged,omitempty"`
}

type Config struct {
	Paths     Paths
	Providers []Provider
	DryRun    bool
	Force     bool
}

type Manager struct {
	paths     Paths
	providers []Provider
	dryRun    bool
	force     bool
}

func New(c Config) *Manager {
	if c.Paths.CanonicalFile == "" {
		c.Paths = DefaultPaths()
	}
	if c.Providers == nil {
		c.Providers = DefaultProviders(c.Paths)
	}
	return &Manager{paths: c.Paths, providers: c.Providers, dryRun: c.DryRun, force: c.Force}
}

func (m *Manager) Providers() []Provider { return m.providers }

func (m *Manager) announce(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if m.dryRun {
		output.Info("would " + msg)
	} else {
		output.Info(msg)
	}
}

// status decides how one canonical server stands in one provider. It is the
// single place the sync policy lives; List renders it and Sync acts on it.
func (m *Manager) status(p Provider, name string, s Server, existing map[string]Server, st State) ProviderStatus {
	cur, present := existing[name]
	if present && cur.unmodeled && st.Owns(p.Name(), name) {
		return ProviderStatus{
			Status: StatusConflict,
			Detail: "provider entry contains fields arc cannot preserve",
		}
	}
	if !s.AppliesTo(p.Name()) {
		return ProviderStatus{Status: StatusExcluded}
	}
	if err := p.Supports(name, s); err != nil {
		return ProviderStatus{Status: StatusUnsupported, Detail: err.Error()}
	}
	if present && cur.unmodeled {
		return ProviderStatus{
			Status: StatusConflict,
			Detail: "provider entry contains fields arc cannot preserve",
		}
	}

	if !s.IsEnabled() && p.OmitsDisabled() {
		if present {
			if !st.Owns(p.Name(), name) {
				return ProviderStatus{
					Status: StatusConflict,
					Detail: "configured by hand while disabled in canonical",
				}
			}
			return ProviderStatus{Status: StatusDrift, Detail: "disabled in canonical but still present"}
		}
		return ProviderStatus{Status: StatusDisabled}
	}
	if !present {
		return ProviderStatus{Status: StatusMissing}
	}
	if p.Normalize(s).Equivalent(cur) {
		return ProviderStatus{Status: StatusOK}
	}
	if st.Owns(p.Name(), name) {
		return ProviderStatus{Status: StatusDrift, Detail: "edited outside arc"}
	}
	return ProviderStatus{Status: StatusConflict, Detail: "configured by hand and differs from canonical"}
}

func (m *Manager) List() (ListResult, error) {
	res := ListResult{CanonicalFile: m.paths.CanonicalFile}
	f, err := Load(m.paths.CanonicalFile)
	if err != nil {
		return res, err
	}
	st, err := LoadState(m.paths.StateFile)
	if err != nil {
		return res, err
	}

	existing := map[string]map[string]Server{}
	for _, p := range m.providers {
		cur, err := p.Read()
		if err != nil {
			output.Warning(fmt.Sprintf("%s: %v", p.Name(), err))
			cur = map[string]Server{}
		}
		existing[p.Name()] = cur
	}

	for _, name := range SortedNames(f.MCPServers) {
		s := f.MCPServers[name]
		entry := ServerEntry{
			Name:      name,
			Type:      s.EffectiveType(),
			Enabled:   s.IsEnabled(),
			EnvRefs:   s.EnvRefs(),
			Providers: map[string]ProviderStatus{},
		}
		for _, p := range m.providers {
			entry.Providers[p.Name()] = m.status(p, name, s, existing[p.Name()], st)
		}
		res.Servers = append(res.Servers, entry)
	}

	for _, p := range m.providers {
		var names []string
		for name := range existing[p.Name()] {
			if _, ok := f.MCPServers[name]; ok {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			res.Unmanaged = append(res.Unmanaged, UnmanagedEntry{Provider: p.Name(), Name: name})
		}
	}
	return res, nil
}

type ProviderSyncResult struct {
	Provider    string            `json:"provider"`
	Path        string            `json:"path"`
	Written     int               `json:"written"`
	Removed     int               `json:"removed"`
	Conflicts   []string          `json:"conflicts,omitempty"`
	Unsupported map[string]string `json:"unsupported,omitempty"`
	Excluded    []string          `json:"excluded,omitempty"`
	Error       string            `json:"error,omitempty"`
}

type SyncResult struct {
	Providers []ProviderSyncResult `json:"providers"`
}

func (r SyncResult) Conflicts() int {
	var n int
	for _, p := range r.Providers {
		n += len(p.Conflicts)
	}
	return n
}

func (r SyncResult) Failures() int {
	var n int
	for _, p := range r.Providers {
		if p.Error != "" {
			n++
		}
	}
	return n
}

// Sync renders canonical into every provider. It is one-way by design:
// ~/ai/mcp.json is the source of truth, and anything arc did not write is left
// for manual review rather than silently absorbed.
func (m *Manager) Sync() (SyncResult, error) {
	f, err := Load(m.paths.CanonicalFile)
	if err != nil {
		return SyncResult{}, err
	}
	return m.syncFile(f)
}

// syncFile is Sync against an explicit canonical set, so Add and Remove can
// report what a --dry-run would do instead of re-reading the unchanged file.
func (m *Manager) syncFile(f File) (SyncResult, error) {
	var res SyncResult
	st, err := LoadState(m.paths.StateFile)
	if err != nil {
		return res, err
	}

	for _, name := range SortedNames(f.MCPServers) {
		if err := Validate(name, f.MCPServers[name]); err != nil {
			return res, fmt.Errorf("%s: %w (run 'arc mcp validate')", m.paths.CanonicalFile, err)
		}
	}

	stateChanged := false
	for _, p := range m.providers {
		pr := ProviderSyncResult{Provider: p.Name(), Path: p.ConfigPath()}

		existing, err := p.Read()
		if err != nil {
			pr.Error = err.Error()
			output.Warning(fmt.Sprintf("%s: %v", p.Name(), err))
			res.Providers = append(res.Providers, pr)
			continue
		}

		desired := map[string]Server{}
		for _, name := range SortedNames(f.MCPServers) {
			s := f.MCPServers[name]
			status := m.status(p, name, s, existing, st)
			switch status.Status {
			case StatusExcluded:
				pr.Excluded = append(pr.Excluded, name)
				continue
			case StatusUnsupported:
				if pr.Unsupported == nil {
					pr.Unsupported = map[string]string{}
				}
				pr.Unsupported[name] = status.Detail
				continue
			case StatusDisabled:
				continue
			case StatusConflict:
				if !m.force {
					pr.Conflicts = append(pr.Conflicts, name)
					continue
				}
				m.announce("overwrite conflicting %s/%s (--force)", p.Name(), name)
			case StatusMissing:
				m.announce("add %s/%s", p.Name(), name)
			case StatusDrift:
				m.announce("update %s/%s", p.Name(), name)
				if !s.IsEnabled() && p.OmitsDisabled() {
					continue
				}
			}
			desired[name] = s
		}

		owned := st.Owned(p.Name())
		for _, name := range owned {
			if _, keep := desired[name]; keep {
				continue
			}
			if _, present := existing[name]; !present {
				continue
			}
			m.announce("remove %s/%s (no longer in canonical)", p.Name(), name)
			pr.Removed++
		}
		pr.Written = len(desired)

		if !m.dryRun {
			if err := p.Write(desired, owned); err != nil {
				pr.Error = err.Error()
				output.Warning(fmt.Sprintf("%s: %v", p.Name(), err))
				res.Providers = append(res.Providers, pr)
				continue
			}
			st.SetOwned(p.Name(), keysOf(desired))
			stateChanged = true
		}
		res.Providers = append(res.Providers, pr)
	}

	if stateChanged && !m.dryRun {
		if err := SaveState(m.paths.StateFile, st); err != nil {
			return res, fmt.Errorf("save %s: %w", m.paths.StateFile, err)
		}
	}
	return res, nil
}

// Add writes a server into canonical and immediately syncs it out, matching how
// `arc skills add` links a new skill everywhere in one step.
func (m *Manager) Add(name string, s Server, force bool) (SyncResult, error) {
	var res SyncResult
	if err := Validate(name, s); err != nil {
		return res, err
	}
	f, err := Load(m.paths.CanonicalFile)
	if err != nil {
		return res, err
	}
	if _, exists := f.MCPServers[name]; exists && !force {
		return res, fmt.Errorf("server %q already exists in %s (use --force to replace)", name, m.paths.CanonicalFile)
	}
	f.MCPServers[name] = s

	m.announce("write %s to %s", name, m.paths.CanonicalFile)
	if !m.dryRun {
		if err := Save(m.paths.CanonicalFile, f); err != nil {
			return res, err
		}
	}
	return m.syncFile(f)
}

// Remove deletes a server from canonical. The follow-up sync sweeps it out of
// every provider that arc put it in; hand-configured copies are left alone.
func (m *Manager) Remove(name string) (SyncResult, error) {
	var res SyncResult
	f, err := Load(m.paths.CanonicalFile)
	if err != nil {
		return res, err
	}
	if _, exists := f.MCPServers[name]; !exists {
		return res, fmt.Errorf("server %q not found in %s", name, m.paths.CanonicalFile)
	}
	delete(f.MCPServers, name)

	m.announce("remove %s from %s", name, m.paths.CanonicalFile)
	if !m.dryRun {
		if err := Save(m.paths.CanonicalFile, f); err != nil {
			return res, err
		}
	}
	return m.syncFile(f)
}

type ImportedServer struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Reason   string `json:"reason,omitempty"`
}

type ImportResult struct {
	CanonicalFile string           `json:"canonical_file"`
	Added         []ImportedServer `json:"added,omitempty"`
	Skipped       []ImportedServer `json:"skipped,omitempty"`
	Conflicts     []ImportedServer `json:"conflicts,omitempty"`
	Rejected      []ImportedServer `json:"rejected,omitempty"`
}

// Import seeds canonical from whatever is already configured in the providers.
// It is the migration path onto arc: without it, adopting a canonical store
// means retyping every server by hand.
//
// Import never overwrites canonical. A name that already exists is either
// identical (skipped) or reported as a conflict for manual reconciliation.
func (m *Manager) Import() (ImportResult, error) {
	res := ImportResult{CanonicalFile: m.paths.CanonicalFile}
	f, err := Load(m.paths.CanonicalFile)
	if err != nil {
		return res, err
	}

	for _, p := range m.providers {
		existing, err := p.Read()
		if err != nil {
			output.Warning(fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		for _, name := range SortedNames(existing) {
			s := existing[name]
			if s.unmodeled {
				res.Rejected = append(res.Rejected, ImportedServer{
					Name: name, Provider: p.Name(),
					Reason: "provider entry contains fields arc cannot preserve",
				})
				continue
			}
			if err := Validate(name, s); err != nil {
				// Most often an inline credential: importing it would copy a
				// secret into a file meant to be shared.
				res.Rejected = append(res.Rejected, ImportedServer{Name: name, Provider: p.Name(), Reason: err.Error()})
				continue
			}
			if cur, exists := f.MCPServers[name]; exists {
				if p.Normalize(cur).Equivalent(s) {
					res.Skipped = append(res.Skipped, ImportedServer{Name: name, Provider: p.Name()})
				} else {
					res.Conflicts = append(res.Conflicts, ImportedServer{
						Name: name, Provider: p.Name(),
						Reason: "already in canonical with a different definition",
					})
				}
				continue
			}
			m.announce("import %s from %s", name, p.Name())
			f.MCPServers[name] = s
			res.Added = append(res.Added, ImportedServer{Name: name, Provider: p.Name()})
		}
	}

	if len(res.Added) > 0 && !m.dryRun {
		if err := Save(m.paths.CanonicalFile, f); err != nil {
			return res, err
		}
	}
	return res, nil
}

type Issue struct {
	Server   string `json:"server"`
	Provider string `json:"provider,omitempty"`
	Error    string `json:"error"`
	Fatal    bool   `json:"fatal"`
}

// Validate checks canonical against the schema (fatal) and against each
// provider's dialect (not fatal — a server that only Codex cannot express is a
// legitimate thing to have).
func (m *Manager) Validate() ([]Issue, error) {
	var issues []Issue
	f, err := Load(m.paths.CanonicalFile)
	if err != nil {
		return nil, err
	}
	for _, name := range SortedNames(f.MCPServers) {
		s := f.MCPServers[name]
		if err := Validate(name, s); err != nil {
			issues = append(issues, Issue{Server: name, Error: err.Error(), Fatal: true})
			continue
		}
		for _, p := range m.providers {
			if !s.AppliesTo(p.Name()) {
				continue
			}
			if err := p.Supports(name, s); err != nil {
				issues = append(issues, Issue{Server: name, Provider: p.Name(), Error: err.Error()})
			}
		}
		for _, ref := range s.EnvRefs() {
			if _, ok := os.LookupEnv(ref); !ok {
				issues = append(issues, Issue{
					Server: name,
					Error:  fmt.Sprintf("references $%s, which is not set in this shell", ref),
				})
			}
		}
	}
	return issues, nil
}

func keysOf(m map[string]Server) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
