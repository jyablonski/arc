package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jyablonski/arc/internal/output"
)

func currentUnix() int64 { return time.Now().Unix() }

type RulesEntry struct {
	Provider string `json:"provider"`
	Target   string `json:"target"`
	Status   Status `json:"status"`
}

type RulesResult struct {
	Canonical string       `json:"canonical"`
	Providers []RulesEntry `json:"providers"`
}

func (m *Manager) SyncRules() (int, error) {
	canonical := m.paths.RulesFile
	if _, err := os.Stat(canonical); err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("canonical %s does not exist; create it first", canonical)
		}
		return 0, err
	}
	var conflicts int
	for _, p := range m.providers {
		if p.RulesFile == "" {
			continue
		}
		target := p.RulesFile
		parent := filepath.Dir(target)
		if err := m.mkdirAll(parent, 0o755); err != nil {
			output.Warning(fmt.Sprintf("%s: mkdir %s: %v", p.Name, parent, err))
			continue
		}
		info, err := os.Lstat(target)
		if err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				output.Warning(fmt.Sprintf("%s: %s is a real file (manual review)", p.Name, target))
				conflicts++
				continue
			}
			linkTarget, _ := os.Readlink(target)
			resolved := linkTarget
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(parent, resolved)
			}
			canonicalAbs, _ := filepath.Abs(canonical)
			resolvedAbs, _ := filepath.Abs(resolved)
			if resolvedAbs == canonicalAbs {
				output.Info(fmt.Sprintf("ok: %s -> %s", target, canonical))
				continue
			}
			m.announce("replace stale symlink %s (was -> %s)", target, linkTarget)
			if !m.dryRun {
				if rerr := m.fs.Remove(target); rerr != nil {
					output.Warning(fmt.Sprintf("%s: remove stale %s: %v", p.Name, target, rerr))
					continue
				}
				if serr := m.fs.Symlink(canonical, target); serr != nil {
					output.Warning(fmt.Sprintf("%s: symlink %s: %v", p.Name, target, serr))
					continue
				}
			}
			continue
		}
		if !os.IsNotExist(err) {
			output.Warning(fmt.Sprintf("%s: lstat %s: %v", p.Name, target, err))
			continue
		}
		m.announce("create symlink %s -> %s", target, canonical)
		if !m.dryRun {
			if serr := m.fs.Symlink(canonical, target); serr != nil {
				output.Warning(fmt.Sprintf("%s: symlink %s: %v", p.Name, target, serr))
				continue
			}
		}
	}
	return conflicts, nil
}

func (m *Manager) StatusRules() RulesResult {
	res := RulesResult{Canonical: m.paths.RulesFile}
	for _, p := range m.providers {
		if p.RulesFile == "" {
			continue
		}
		res.Providers = append(res.Providers, RulesEntry{
			Provider: p.Name,
			Target:   p.RulesFile,
			Status:   slotStatus(p.RulesFile, m.paths.RulesFile),
		})
	}
	return res
}
